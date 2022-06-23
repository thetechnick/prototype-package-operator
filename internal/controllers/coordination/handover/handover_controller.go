package handover

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers"
	"github.com/thetechnick/package-operator/internal/controllers/coordination"
	"github.com/thetechnick/package-operator/internal/dynamicwatcher"
)

// Generic reconciler for both Handover and ClusterHandover objects.
type GenericHandoverController struct {
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
	Reconcile(ctx context.Context, handover genericHandover) (ctrl.Result, error)
}

func NewHandoverController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericHandoverController {
	return NewGenericHandoverController(
		coordinationv1alpha1.GroupVersion.WithKind("Handover"),
		c, log, scheme, dynamicClient, discoveryClient,
	)
}

func NewClusterHandoverController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericHandoverController {
	return NewGenericHandoverController(
		coordinationv1alpha1.GroupVersion.WithKind("ClusterHandover"),
		c, log, scheme, dynamicClient, discoveryClient,
	)
}

func NewGenericHandoverController(
	gvk schema.GroupVersionKind,
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericHandoverController {
	return &GenericHandoverController{
		gvk: gvk,

		client:          c,
		log:             log,
		scheme:          scheme,
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
		reconciler: []reconciler{
			&relabelReconciler{client: c},
		},
	}
}

func (c *GenericHandoverController) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := c.log.WithValues(c.gvk.Kind, req.NamespacedName.String())
	ctx = controllers.ContextWithLogger(ctx, log)

	defer log.Info("reconciled")

	handover := c.newOperand()
	if err := c.client.Get(ctx, req.NamespacedName, handover.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !handover.ClientObject().GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, c.handleDeletion(ctx, handover)
	}

	if err := c.ensureCacheFinalizer(ctx, handover); err != nil {
		return ctrl.Result{}, err
	}

	if err := c.ensureWatch(ctx, handover); err != nil {
		return ctrl.Result{}, err
	}

	var (
		res ctrl.Result
		err error
	)
	for _, r := range c.reconciler {
		res, err = r.Reconcile(ctx, handover)
		if err != nil || !res.IsZero() {
			break
		}
	}
	if err != nil {
		return res, err
	}

	handover.UpdatePhase()
	return res, c.client.Status().Update(ctx, handover.ClientObject())
}

func (c *GenericHandoverController) SetupWithManager(
	mgr ctrl.Manager) error {
	c.dw = dynamicwatcher.New(
		c.log, c.scheme, c.client.RESTMapper(), c.dynamicClient)
	t := c.newOperand().ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(t).
		Watches(c.dw, &dynamicwatcher.EnqueueWatchingObjects{
			WatcherType:      t,
			WatcherRefGetter: c.dw,
			ClusterScoped:    strings.HasPrefix(c.gvk.Kind, "Cluster"),
		}).
		Complete(c)
}

func (c *GenericHandoverController) newOperand() genericHandover {
	obj, err := c.scheme.New(c.gvk)
	if err != nil {
		panic(err)
	}
	switch o := obj.(type) {
	case *coordinationv1alpha1.Handover:
		return &GenericHandover{Handover: *o}
	case *coordinationv1alpha1.ClusterHandover:
		return &GenericClusterHandover{ClusterHandover: *o}
	}
	panic("unsupported gvk")
}

func (c *GenericHandoverController) handleDeletion(
	ctx context.Context, handover genericHandover,
) error {
	return controllers.HandleCommonDeletion(
		ctx, handover.ClientObject(), c.client, c.dw, coordination.CacheFinalizer)
}

// ensures the cache finalizer is set on the given object
func (c *GenericHandoverController) ensureCacheFinalizer(
	ctx context.Context, handover genericHandover,
) error {
	return controllers.EnsureCommonFinalizer(
		ctx, handover.ClientObject(), c.client, coordination.CacheFinalizer)
}

// ensures the cache is watching the targetAPI
func (c *GenericHandoverController) ensureWatch(
	ctx context.Context, handover genericHandover,
) error {
	gvk, objType, _ := coordination.UnstructuredFromTargetAPI(
		handover.GetTargetAPI())

	if err := c.dw.Watch(handover.ClientObject(), objType); err != nil {
		return fmt.Errorf("watching %s: %w", gvk, err)
	}
	return nil
}
