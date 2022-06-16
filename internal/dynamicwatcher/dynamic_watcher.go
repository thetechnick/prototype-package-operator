package dynamicwatcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// DynamicWatcher is able to dynamically allocate new watches for arbitrary objects.
// Multiple watches to the same resource, will be de-duplicated.
type DynamicWatcher struct {
	log        logr.Logger
	scheme     *runtime.Scheme
	restMapper meta.RESTMapper
	client     dynamic.Interface

	sinksLock sync.RWMutex
	sinks     []watchSink

	mu                 sync.Mutex
	informers          map[namespacedGKV]chan<- struct{}
	informerReferences map[namespacedGKV]map[OwnerRef]struct{}
}

var _ source.Source = (*DynamicWatcher)(nil)

type namespacedGKV struct {
	schema.GroupVersionKind
	Namespace string
}

type OwnerRef struct {
	UID       types.UID
	Group     string
	Kind      string
	Name      string
	Namespace string
}

type watchSink struct {
	ctx        context.Context
	handler    handler.EventHandler
	queue      workqueue.RateLimitingInterface
	predicates []predicate.Predicate
}

func New(
	log logr.Logger,
	scheme *runtime.Scheme,
	restMapper meta.RESTMapper,
	client dynamic.Interface,
) *DynamicWatcher {
	return &DynamicWatcher{
		log:        log,
		scheme:     scheme,
		restMapper: restMapper,
		client:     client,

		informers:          map[namespacedGKV]chan<- struct{}{},
		informerReferences: map[namespacedGKV]map[OwnerRef]struct{}{},
	}
}

func (dw *DynamicWatcher) OwnersForNamespacedGKV(ngvk namespacedGKV) []OwnerRef {
	dw.mu.Lock()
	defer dw.mu.Unlock()
	var ownerRefs []OwnerRef
	for ownerRef := range dw.informerReferences[ngvk] {
		ownerRefs = append(ownerRefs, ownerRef)
	}
	return ownerRefs
}

// Starts this event source.
func (dw *DynamicWatcher) Start(
	ctx context.Context,
	handler handler.EventHandler,
	queue workqueue.RateLimitingInterface,
	predicates ...predicate.Predicate,
) error {
	dw.sinksLock.Lock()
	defer dw.sinksLock.Unlock()
	dw.sinks = append(dw.sinks, watchSink{
		ctx:        ctx,
		handler:    handler,
		queue:      queue,
		predicates: predicates,
	})
	return nil
}

func (dw *DynamicWatcher) String() string {
	return "DynamicWatcher"
}

// Watch the given object type and associate the watch with the given owner.
func (dw *DynamicWatcher) Watch(owner client.Object, obj runtime.Object) error {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	gvk, err := apiutil.GVKForObject(obj, dw.scheme)
	if err != nil {
		return fmt.Errorf("get GVK for object: %w", err)
	}
	ngvk := namespacedGKV{
		Namespace:        owner.GetNamespace(),
		GroupVersionKind: gvk,
	}

	// Check if informer is already registered.
	ownerRef, err := dw.ownerRef(owner)
	if err != nil {
		return err
	}
	if _, ok := dw.informers[ngvk]; !ok {
		dw.informerReferences[ngvk] = map[OwnerRef]struct{}{}
	}
	dw.informerReferences[ngvk][ownerRef] = struct{}{}
	if _, ok := dw.informers[ngvk]; ok {
		dw.log.Info(
			"reusing existing watcher",
			"owner", schema.GroupKind{Group: ownerRef.Group, Kind: ownerRef.Kind},
			"for", schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, "namespace", owner.GetNamespace())
		return nil
	}

	// Adding new watcher.
	informerStopChannel := make(chan struct{})
	dw.informers[ngvk] = informerStopChannel
	dw.log.Info("adding new watcher",
		"owner", schema.GroupKind{Group: ownerRef.Group, Kind: ownerRef.Kind},
		"for", schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, "namespace", owner.GetNamespace())

	// Build client
	restMapping, err := dw.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("unable to map object to rest endpoint: %w", err)
	}
	client := dw.client.Resource(restMapping.Resource)

	ctx := context.Background()
	var informer cache.SharedIndexInformer
	if owner.GetNamespace() == "" {
		informer = cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return client.List(ctx, options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return client.Watch(ctx, options)
				},
			},
			obj, 10*time.Hour, nil)
	} else {
		informer = cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return client.Namespace(owner.GetNamespace()).List(ctx, options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return client.Namespace(owner.GetNamespace()).Watch(ctx, options)
				},
			},
			obj, 10*time.Hour, nil)
	}
	s := source.Informer{
		Informer: informer,
	}

	dw.sinksLock.Lock()
	defer dw.sinksLock.Unlock()
	for _, sink := range dw.sinks {
		if err := s.Start(sink.ctx, sink.handler, sink.queue, sink.predicates...); err != nil {
			return err
		}
	}
	go informer.Run(informerStopChannel)
	return nil
}

// Free all watches associated with the given owner.
func (dw *DynamicWatcher) Free(owner client.Object) error {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	ownerRef, err := dw.ownerRef(owner)
	if err != nil {
		return err
	}

	for gvk, refs := range dw.informerReferences {
		if _, ok := refs[ownerRef]; ok {
			delete(refs, ownerRef)

			if len(refs) == 0 {
				close(dw.informers[gvk])
				delete(dw.informers, gvk)
				dw.log.Info("releasing watcher",
					"kind", gvk.Kind, "group", gvk.Group, "namespace", owner.GetNamespace())
			}
		}
	}
	return nil
}

func (dw *DynamicWatcher) ownerRef(owner client.Object) (OwnerRef, error) {
	ownerGVK, err := apiutil.GVKForObject(owner, dw.scheme)
	if err != nil {
		return OwnerRef{}, fmt.Errorf("get GVK for object: %w", err)
	}

	return OwnerRef{
		UID:       owner.GetUID(),
		Group:     ownerGVK.Group,
		Kind:      ownerGVK.Kind,
		Name:      owner.GetName(),
		Namespace: owner.GetNamespace(),
	}, nil
}
