package objectset

import (
	"context"

	"github.com/go-logr/logr"
	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers"
	"github.com/thetechnick/package-operator/internal/controllers/packages"
	"github.com/thetechnick/package-operator/internal/dynamicwatcher"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

// Generic reconciler for both ObjectSet and ClusterObjectSet objects.
type GenericObjectSetController struct {
	gvk schema.GroupVersionKind

	client          client.Client
	log             logr.Logger
	scheme          *runtime.Scheme
	dynamicClient   dynamic.Interface
	discoveryClient *discovery.DiscoveryClient
	reconciler      []reconciler

	dw *dynamicwatcher.DynamicWatcher
}

type reconciler interface {
	Reconcile(ctx context.Context, objectSet genericObjectSet) (ctrl.Result, error)
}

func NewObjectSetController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericObjectSetController {
	return NewGenericObjectSetController(
		packagesv1alpha1.GroupVersion.WithKind("ObjectSet"),
		c, log, scheme, dynamicClient, discoveryClient,
	)
}

func NewClusterObjectSetController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericObjectSetController {
	return NewGenericObjectSetController(
		packagesv1alpha1.GroupVersion.WithKind("ClusterObjectSet"),
		c, log, scheme, dynamicClient, discoveryClient,
	)
}

func NewGenericObjectSetController(
	gvk schema.GroupVersionKind,
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericObjectSetController {
	dw := dynamicwatcher.New(
		log, scheme, c.RESTMapper(), dynamicClient)

	return &GenericObjectSetController{
		gvk: gvk,

		client:          c,
		log:             log,
		scheme:          scheme,
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
		dw:              dw,
		reconciler: []reconciler{
			&ArchivedObjectSetReconciler{
				client: c,
				dw:     dw,
			},
			&ObjectSetDependencyReconciler{
				client:          c,
				discoveryClient: discoveryClient,
			},
			&ObjectSetPhaseReconciler{
				client: c,
				scheme: scheme,
				dw:     dw,
			},
		},
	}
}

func (c *GenericObjectSetController) SetupWithManager(
	mgr ctrl.Manager) error {

	t := c.newOperand().ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(t).
		Watches(c.dw, &handler.EnqueueRequestForOwner{
			OwnerType:    t,
			IsController: false,
		}).
		Complete(c)
}

func (c *GenericObjectSetController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("ObjectSet", req.NamespacedName.String())
	ctx = controllers.ContextWithLogger(ctx, log)

	objectSet := c.newOperand()
	if err := c.client.Get(
		ctx, req.NamespacedName, objectSet.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !objectSet.ClientObject().GetDeletionTimestamp().IsZero() {
		// ObjectSet was deleted.
		return ctrl.Result{}, c.handleDeletion(ctx, objectSet)
	}

	if err := c.ensureCacheFinalizer(ctx, objectSet); err != nil {
		return ctrl.Result{}, err
	}

	var (
		res ctrl.Result
		err error
	)
	for _, r := range c.reconciler {
		res, err = r.Reconcile(ctx, objectSet)
		if err != nil || !res.IsZero() {
			break
		}
	}
	if err != nil {
		return res, err
	}

	objectSet.SetStatusPausedFor(objectSet.GetPausedFor()) // report paused objects via status
	objectSet.UpdatePhase()
	return res, c.client.Status().Update(ctx, objectSet.ClientObject())
}

func (c *GenericObjectSetController) newOperand() genericObjectSet {
	obj, err := c.scheme.New(c.gvk)
	if err != nil {
		panic(err)
	}

	switch o := obj.(type) {
	case *packagesv1alpha1.ObjectSet:
		return &GenericObjectSet{ObjectSet: *o}
	case *packagesv1alpha1.ClusterObjectSet:
		return &GenericClusterObjectSet{ClusterObjectSet: *o}
	}
	panic("unsupported gvk")
}

func (c *GenericObjectSetController) handleDeletion(
	ctx context.Context, objectSet genericObjectSet,
) error {
	// TODO: Reverse phases and delete objects still owned by
	// this ObjectSet instance

	return controllers.HandleCommonDeletion(ctx, objectSet.ClientObject(), c.client, c.dw, packages.CacheFinalizer)
}

// ensures the cache finalizer is set on the given object
func (c *GenericObjectSetController) ensureCacheFinalizer(
	ctx context.Context, objectSet genericObjectSet,
) error {
	return controllers.EnsureCommonFinalizer(ctx, objectSet.ClientObject(), c.client, packages.CacheFinalizer)
}
