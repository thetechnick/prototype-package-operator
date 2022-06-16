package objectsets

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers"
	"github.com/thetechnick/package-operator/internal/controllers/packages"
	"github.com/thetechnick/package-operator/internal/ownerhandling"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Generic reconciler for both ObjectSet and ClusterObjectSet objects.
type GenericObjectSetController struct {
	gvk, phaseGVK schema.GroupVersionKind

	client          client.Client
	log             logr.Logger
	scheme          *runtime.Scheme
	teardownHandler teardownHandler
	reconciler      []reconciler

	dw dynamicWatcher
}

type reconciler interface {
	Reconcile(ctx context.Context, objectSet genericObjectSet) (ctrl.Result, error)
}

type dynamicWatcher interface {
	source.Source
	Free(obj client.Object) error
	Watch(owner client.Object, obj runtime.Object) error
}

func NewObjectSetController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dw dynamicWatcher,
) *GenericObjectSetController {
	return NewGenericObjectSetController(
		packagesv1alpha1.GroupVersion.WithKind("ObjectSet"),
		packagesv1alpha1.GroupVersion.WithKind("ObjectSetPhase"),
		c, log, scheme, dw,
	)
}

func NewClusterObjectSetController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dw dynamicWatcher,
) *GenericObjectSetController {
	return NewGenericObjectSetController(
		packagesv1alpha1.GroupVersion.WithKind("ClusterObjectSet"),
		packagesv1alpha1.GroupVersion.WithKind("ClusterObjectSetPhase"),
		c, log, scheme, dw,
	)
}

func NewGenericObjectSetController(
	gvk schema.GroupVersionKind,
	phaseGVK schema.GroupVersionKind,
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dw dynamicWatcher,
) *GenericObjectSetController {
	controller := &GenericObjectSetController{
		gvk:      gvk,
		phaseGVK: phaseGVK,

		client: c,
		log:    log,
		scheme: scheme,
		dw:     dw,
	}

	controller.teardownHandler = NewTeardownHandler(c, dw, controller.newPhase)

	controller.reconciler = []reconciler{
		&ArchivedObjectSetReconciler{
			dw:              dw,
			teardownHandler: controller.teardownHandler,
		},
		&ObjectSetPhaseReconciler{
			client:            c,
			scheme:            scheme,
			dw:                dw,
			newObjectSetPhase: controller.newPhase,
			phaseReconciler:   packages.NewPhaseReconciler(dw, c, scheme, ownerhandling.Native),
		},
	}

	return controller
}

func (c *GenericObjectSetController) SetupWithManager(
	mgr ctrl.Manager) error {

	t := c.newOperand().ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(t).
		Owns(c.newPhase().ClientObject()).
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

func (c *GenericObjectSetController) newPhase() genericObjectSetPhase {
	obj, err := c.scheme.New(c.phaseGVK)
	if err != nil {
		panic(err)
	}

	switch o := obj.(type) {
	case *packagesv1alpha1.ObjectSetPhase:
		return &GenericObjectSetPhase{ObjectSetPhase: *o}
	case *packagesv1alpha1.ClusterObjectSetPhase:
		return &GenericClusterObjectSetPhase{ClusterObjectSetPhase: *o}
	}
	panic("unsupported gvk")
}

func (c *GenericObjectSetController) handleDeletion(
	ctx context.Context, objectSet genericObjectSet,
) error {
	done, err := c.teardownHandler.Teardown(ctx, objectSet)
	if err != nil {
		return fmt.Errorf("error tearing down during deletion: %w", err)
	}

	if !done {
		// dont remove finalizers before deletion is done
		return nil
	}

	return controllers.HandleCommonDeletion(ctx, objectSet.ClientObject(), c.client, c.dw, packages.CacheFinalizer)
}

// ensures the cache finalizer is set on the given object
func (c *GenericObjectSetController) ensureCacheFinalizer(
	ctx context.Context, objectSet genericObjectSet,
) error {
	return controllers.EnsureCommonFinalizer(ctx, objectSet.ClientObject(), c.client, packages.CacheFinalizer)
}
