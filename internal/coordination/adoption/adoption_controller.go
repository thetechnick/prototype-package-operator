package adoption

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	"github.com/thetechnick/package-operator/internal/dynamicwatcher"
)

type operand interface {
	coordinationv1alpha1.Adoption | coordinationv1alpha1.ClusterAdoption
}

type operandPtr[O any] interface {
	client.Object
	*O
}

const (
	cacheFinalizer = "coordination.thetechnick.ninja/cache"
)

// Generic reconciler for both Adoption and ClusterAdoption objects.
// An adoption controller ensures selected objects always have a specific label set.
type GenericAdoptionController[T operandPtr[O], O operand] struct {
	client          client.Client
	log             logr.Logger
	scheme          *runtime.Scheme
	dynamicClient   dynamic.Interface
	discoveryClient *discovery.DiscoveryClient
	reconciler      []reconciler[T]

	dw *dynamicwatcher.DynamicWatcher
}

type reconciler[T any] interface {
	Reconcile(ctx context.Context, adoption T) (ctrl.Result, error)
}

func NewAdoptionController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericAdoptionController[*coordinationv1alpha1.Adoption, coordinationv1alpha1.Adoption] {
	return NewGenericAdoptionController(
		coordinationv1alpha1.Adoption{},
		c, log, scheme, dynamicClient, discoveryClient,
	)
}

func NewClusterAdoptionController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericAdoptionController[*coordinationv1alpha1.ClusterAdoption, coordinationv1alpha1.ClusterAdoption] {
	return NewGenericAdoptionController(
		coordinationv1alpha1.ClusterAdoption{},
		c, log, scheme, dynamicClient, discoveryClient,
	)
}

func NewGenericAdoptionController[T operandPtr[O], O operand](
	o O,
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericAdoptionController[T, O] {
	return &GenericAdoptionController[T, O]{
		client:          c,
		log:             log,
		scheme:          scheme,
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,

		reconciler: []reconciler[T]{
			&StaticAdoptionReconciler[T, O]{client: c},
			&RoundRobinAdoptionReconciler[T, O]{client: c},
		},
	}
}

func (c *GenericAdoptionController[T, O]) SetupWithManager(
	mgr ctrl.Manager) error {
	c.dw = dynamicwatcher.New(
		c.log, c.scheme, c.client.RESTMapper(), c.dynamicClient)
	t := c.newOperand()

	return ctrl.NewControllerManagedBy(mgr).
		For(t).
		Watches(c.dw, &dynamicwatcher.EnqueueWatchingObjects{
			WatcherType:      t,
			WatcherRefGetter: c.dw,
		}).
		Complete(c)
}

func (c *GenericAdoptionController[T, O]) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	adoption := c.newOperand()
	if err := c.client.Get(
		ctx, req.NamespacedName, adoption); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !adoption.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, c.handleDeletion(ctx, adoption)
	}

	if err := c.ensureCacheFinalizer(ctx, adoption); err != nil {
		return ctrl.Result{}, err
	}

	if err := c.ensureWatch(ctx, adoption); err != nil {
		return ctrl.Result{}, err
	}

	for _, r := range c.reconciler {
		res, err := r.Reconcile(ctx, adoption)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !res.IsZero() {
			return res, nil
		}
	}

	// set generic success status
	setStatus(adoption)

	return ctrl.Result{}, c.client.Status().Update(ctx, adoption)
}

func (c *GenericAdoptionController[T, O]) newOperand() T {
	var o O
	return T(&o)
}

func (c *GenericAdoptionController[T, O]) handleDeletion(
	ctx context.Context, adoption T,
) error {
	if controllerutil.ContainsFinalizer(adoption, cacheFinalizer) {
		controllerutil.RemoveFinalizer(adoption, cacheFinalizer)

		if err := c.client.Update(ctx, adoption); err != nil {
			return fmt.Errorf("removing finalizer: %w", err)
		}
	}

	if err := c.dw.Free(adoption); err != nil {
		return fmt.Errorf("free cache: %w", err)
	}
	return nil
}

// ensures the cache finalizer is set on the given object
func (c *GenericAdoptionController[T, O]) ensureCacheFinalizer(
	ctx context.Context, adoption T) error {
	if !controllerutil.ContainsFinalizer(
		adoption, cacheFinalizer) {
		controllerutil.AddFinalizer(adoption, cacheFinalizer)
		if err := c.client.Update(ctx, adoption); err != nil {
			return fmt.Errorf("adding finalizer: %w", err)
		}
	}
	return nil
}

// ensures the cache is watching the targetAPI
func (c *GenericAdoptionController[T, O]) ensureWatch(
	ctx context.Context, adoption T,
) error {
	gvk, objType, _ := unstructuredFromTargetAPI(getTargetAPI(adoption))

	if err := c.dw.Watch(adoption, objType); err != nil {
		return fmt.Errorf("watching %s: %w", gvk, err)
	}
	return nil
}
