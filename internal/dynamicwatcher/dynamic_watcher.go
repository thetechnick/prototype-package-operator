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

type namespacedGKV struct {
	schema.GroupVersionKind
	Namespace string
}

var _ source.Source = (*DynamicWatcher)(nil)

type DynamicWatcher struct {
	log        logr.Logger
	scheme     *runtime.Scheme
	restMapper meta.RESTMapper
	client     dynamic.Interface

	ctx        context.Context
	handler    handler.EventHandler
	queue      workqueue.RateLimitingInterface
	predicates []predicate.Predicate

	mu                 sync.Mutex
	informers          map[namespacedGKV]chan<- struct{}
	informerReferences map[namespacedGKV]map[types.UID]struct{}
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
		informerReferences: map[namespacedGKV]map[types.UID]struct{}{},
	}
}

// Starts this event source.
func (dw *DynamicWatcher) Start(ctx context.Context, handler handler.EventHandler, queue workqueue.RateLimitingInterface, predicates ...predicate.Predicate) error {
	dw.ctx = ctx
	dw.handler = handler
	dw.queue = queue
	dw.predicates = predicates
	return nil
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
	if _, ok := dw.informers[ngvk]; !ok {
		dw.informerReferences[ngvk] = map[types.UID]struct{}{}
	}
	dw.informerReferences[ngvk][owner.GetUID()] = struct{}{}
	if _, ok := dw.informers[ngvk]; ok {
		dw.log.Info("reusing existing watcher",
			"kind", gvk.Kind, "group", gvk.Group, "namespace", owner.GetNamespace())
		return nil
	}

	// Adding new watcher.
	informerStopChannel := make(chan struct{})
	dw.informers[ngvk] = informerStopChannel
	dw.log.Info("adding new watcher",
		"kind", gvk.Kind, "group", gvk.Group, "namespace", owner.GetNamespace())

	// Build client
	restMapping, err := dw.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("unable to map object to rest endpoint: %w", err)
	}
	client := dw.client.Resource(restMapping.Resource)

	ctx := dw.ctx
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return client.Namespace(owner.GetNamespace()).List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return client.Namespace(owner.GetNamespace()).Watch(ctx, options)
			},
		},
		obj, 10*time.Hour, nil)
	s := source.Informer{
		Informer: informer,
	}

	if err := s.Start(ctx, dw.handler, dw.queue, dw.predicates...); err != nil {
		return err
	}
	go informer.Run(informerStopChannel)
	return nil
}

// Free all watches associated with the given owner.
func (dw *DynamicWatcher) Free(owner client.Object) error {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	uid := owner.GetUID()

	for gvk, refs := range dw.informerReferences {
		if _, ok := refs[uid]; ok {
			delete(refs, uid)

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
