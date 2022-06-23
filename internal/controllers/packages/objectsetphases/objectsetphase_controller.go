package objectsetphases

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers"
	"github.com/thetechnick/package-operator/internal/controllers/packages"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Generic reconciler for both ObjectSetPhase and ClusterObjectSetPhase objects.
type GenericObjectSetPhaseController struct {
	gvk schema.GroupVersionKind

	class                string
	client, targetClient client.Client
	log                  logr.Logger
	scheme               *runtime.Scheme
	reconciler           []reconciler
	ownerStrategy        ownerStrategy

	dw dynamicWatcher
}

type ownerStrategy interface {
	IsOwner(owner, obj metav1.Object) bool
	ReleaseController(obj metav1.Object)
	SetControllerReference(owner, obj metav1.Object, scheme *runtime.Scheme) error
	EnqueueRequestForOwner(ownerType client.Object, isController bool) handler.EventHandler
}

type reconciler interface {
	Reconcile(ctx context.Context, objectSet genericObjectSetPhase) (ctrl.Result, error)
}

type dynamicWatcher interface {
	source.Source
	Free(obj client.Object) error
	Watch(owner client.Object, obj runtime.Object) error
}

func NewObjectSetPhaseController(
	class string, ownerStrategy ownerStrategy,
	c, targetClient client.Client,
	log logr.Logger,
	scheme *runtime.Scheme, dw dynamicWatcher,
) *GenericObjectSetPhaseController {
	return NewGenericObjectSetPhaseController(
		class, ownerStrategy,
		packagesv1alpha1.GroupVersion.WithKind("ObjectSetPhase"),
		c, targetClient, log, scheme, dw,
	)
}

func NewClusterObjectSetPhaseController(
	class string, ownerStrategy ownerStrategy,
	c, targetClient client.Client,
	log logr.Logger,
	scheme *runtime.Scheme, dw dynamicWatcher,
) *GenericObjectSetPhaseController {
	return NewGenericObjectSetPhaseController(
		class, ownerStrategy,
		packagesv1alpha1.GroupVersion.WithKind("ClusterObjectSetPhase"),
		c, targetClient, log, scheme, dw,
	)
}

func NewGenericObjectSetPhaseController(
	class string, ownerStrategy ownerStrategy,
	gvk schema.GroupVersionKind,
	c, targetClient client.Client,
	log logr.Logger,
	scheme *runtime.Scheme, dw dynamicWatcher,
) *GenericObjectSetPhaseController {
	controller := &GenericObjectSetPhaseController{
		gvk:   gvk,
		class: class,

		client:        c,
		targetClient:  targetClient,
		log:           log,
		scheme:        scheme,
		dw:            dw,
		ownerStrategy: ownerStrategy,
	}

	controller.reconciler = []reconciler{
		&PhaseReconciler{
			phaseReconciler: packages.NewPhaseReconciler(
				dw, targetClient, scheme, ownerStrategy),
		},
	}

	return controller
}

func (c *GenericObjectSetPhaseController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("ObjectSetPhase", req.NamespacedName.String())
	ctx = controllers.ContextWithLogger(ctx, log)

	defer log.Info("reconciled")

	objectSetPhase := c.newOperand()
	if err := c.client.Get(
		ctx, req.NamespacedName, objectSetPhase.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if objectSetPhase.GetClass() != c.class {
		return ctrl.Result{}, nil
	}

	if !objectSetPhase.ClientObject().GetDeletionTimestamp().IsZero() {
		// ObjectSetPhase was deleted.
		return ctrl.Result{}, c.handleDeletion(ctx, objectSetPhase)
	}

	if err := c.ensureCacheFinalizer(ctx, objectSetPhase); err != nil {
		return ctrl.Result{}, err
	}

	var (
		res ctrl.Result
		err error
	)
	for _, r := range c.reconciler {
		res, err = r.Reconcile(ctx, objectSetPhase)
		if err != nil || !res.IsZero() {
			break
		}
	}
	if err != nil {
		return res, err
	}

	objectSetPhase.SetStatusPausedFor(objectSetPhase.GetPausedFor()) // report paused objects via status
	return res, c.client.Status().Update(ctx, objectSetPhase.ClientObject())
}

func (c *GenericObjectSetPhaseController) newOperand() genericObjectSetPhase {
	obj, err := c.scheme.New(c.gvk)
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

func (c *GenericObjectSetPhaseController) handleDeletion(
	ctx context.Context, objectSetPhase genericObjectSetPhase,
) error {
	done, err := packages.TeardownPhase(ctx, c.targetClient, objectSetPhase, objectSetPhase.GetPhase())
	if err != nil {
		return fmt.Errorf("tearing down ObjectSetPhase: %w", err)
	}

	if !done {
		// wait till we remove our finalizer
		return nil
	}

	return controllers.HandleCommonDeletion(ctx, objectSetPhase.ClientObject(), c.client, c.dw, packages.CacheFinalizer)
}

// ensures the cache finalizer is set on the given object
func (c *GenericObjectSetPhaseController) ensureCacheFinalizer(
	ctx context.Context, objectSet genericObjectSetPhase,
) error {
	return controllers.EnsureCommonFinalizer(ctx, objectSet.ClientObject(), c.client, packages.CacheFinalizer)
}

func (c *GenericObjectSetPhaseController) SetupWithManager(
	mgr ctrl.Manager) error {

	t := c.newOperand().ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(t).
		Watches(c.dw, c.ownerStrategy.EnqueueRequestForOwner(t, false)).
		Complete(c)
}
