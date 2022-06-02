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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

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
	informerReferences map[namespacedGKV]map[OwnerRef]struct{}
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
	ownerRef, err := dw.ownerRef(owner)
	if err != nil {
		return err
	}
	if _, ok := dw.informers[ngvk]; !ok {
		dw.informerReferences[ngvk] = map[OwnerRef]struct{}{}
	}
	dw.informerReferences[ngvk][ownerRef] = struct{}{}
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

type ownerRefGetter interface {
	OwnersForNamespacedGKV(ngvk namespacedGKV) []OwnerRef
}

type EnqueueWatchingObjects struct {
	WatcherRefGetter ownerRefGetter
	// WatcherType is the type of the Owner object to look for in OwnerReferences.  Only Group and Kind are compared.
	WatcherType   runtime.Object
	ClusterScoped bool

	scheme *runtime.Scheme
	// groupKind is the cached Group and Kind from WatcherType
	groupKind schema.GroupKind
}

func (e *EnqueueWatchingObjects) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.enqueueWatchers(evt.Object, q)
}

func (e *EnqueueWatchingObjects) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.enqueueWatchers(evt.ObjectNew, q)
}

func (e *EnqueueWatchingObjects) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.enqueueWatchers(evt.Object, q)
}

func (e *EnqueueWatchingObjects) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.enqueueWatchers(evt.Object, q)
}

// InjectScheme is called by the Controller to provide a singleton scheme to the EnqueueRequestForOwner.
func (e *EnqueueWatchingObjects) InjectScheme(s *runtime.Scheme) error {
	e.scheme = s
	return e.parseWatcherTypeGroupKind(s)
}

func (e *EnqueueWatchingObjects) enqueueWatchers(obj client.Object, q workqueue.RateLimitingInterface) {
	if obj == nil {
		return
	}

	gvk, err := apiutil.GVKForObject(obj, e.scheme)
	if err != nil {
		// TODO: error reporting?
		panic(err)
	}

	var ngvk namespacedGKV
	if e.ClusterScoped {
		ngvk = namespacedGKV{
			GroupVersionKind: gvk,
		}
	} else {
		ngvk = namespacedGKV{
			GroupVersionKind: gvk,
			Namespace:        obj.GetNamespace(),
		}
	}

	ownerRefs := e.WatcherRefGetter.OwnersForNamespacedGKV(ngvk)
	for _, ownerRef := range ownerRefs {
		if ownerRef.Kind != e.groupKind.Kind ||
			ownerRef.Group != e.groupKind.Group {
			continue
		}

		q.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      ownerRef.Name,
				Namespace: ownerRef.Namespace,
			}})
	}
}

// parseOwnerTypeGroupKind parses the WatcherType into a Group and Kind and caches the result.  Returns false
// if the WatcherType could not be parsed using the scheme.
func (e *EnqueueWatchingObjects) parseWatcherTypeGroupKind(scheme *runtime.Scheme) error {
	// Get the kinds of the type
	kinds, _, err := scheme.ObjectKinds(e.WatcherType)
	if err != nil {
		return err
	}
	// Expect only 1 kind.  If there is more than one kind this is probably an edge case such as ListOptions.
	if len(kinds) != 1 {
		err := fmt.Errorf("Expected exactly 1 kind for WatcherType %T, but found %s kinds", e.WatcherType, kinds)
		return err

	}
	// Cache the Group and Kind for the WatcherType
	e.groupKind = schema.GroupKind{Group: kinds[0].Group, Kind: kinds[0].Kind}
	return nil
}
