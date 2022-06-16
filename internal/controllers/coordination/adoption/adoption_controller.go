package adoption

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// Generic reconciler for both Adoption and ClusterAdoption objects.
// An adoption controller ensures selected objects always have a specific label set.
type GenericAdoptionController struct {
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
	Reconcile(ctx context.Context, adoption genericAdoption) (ctrl.Result, error)
}

func NewAdoptionController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericAdoptionController {
	return newGenericAdoptionController(
		coordinationv1alpha1.GroupVersion.WithKind("Adoption"),
		c, log, scheme, dynamicClient, discoveryClient,
	)
}

func NewClusterAdoptionController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericAdoptionController {
	return newGenericAdoptionController(
		coordinationv1alpha1.GroupVersion.WithKind("ClusterAdoption"),
		c, log, scheme, dynamicClient, discoveryClient,
	)
}

func newGenericAdoptionController(
	gvk schema.GroupVersionKind,
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericAdoptionController {
	return &GenericAdoptionController{
		gvk:             gvk,
		client:          c,
		log:             log,
		scheme:          scheme,
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,

		reconciler: []reconciler{
			&StaticAdoptionReconciler{client: c},
			&RoundRobinAdoptionReconciler{client: c},
		},
	}
}

func (c *GenericAdoptionController) SetupWithManager(
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

func (c *GenericAdoptionController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues(c.gvk.Kind, req.NamespacedName.String())
	ctx = controllers.ContextWithLogger(ctx, log)

	defer log.Info("reconciled")

	adoption := c.newOperand()
	if err := c.client.Get(
		ctx, req.NamespacedName, adoption.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !adoption.ClientObject().GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, c.handleDeletion(ctx, adoption)
	}

	if err := c.ensureCacheFinalizer(ctx, adoption); err != nil {
		return ctrl.Result{}, err
	}

	if err := c.ensureWatch(ctx, adoption); err != nil {
		return ctrl.Result{}, err
	}

	var (
		res ctrl.Result
		err error
	)
	for _, r := range c.reconciler {
		res, err = r.Reconcile(ctx, adoption)
		if err != nil || !res.IsZero() {
			break
		}
	}
	if err != nil {
		return res, err
	}

	// set generic success status
	meta.SetStatusCondition(adoption.GetConditions(), metav1.Condition{
		Type:               coordinationv1alpha1.AdoptionActive,
		Status:             metav1.ConditionTrue,
		Reason:             "Setup",
		Message:            "Controller is setup and adding labels.",
		ObservedGeneration: adoption.ClientObject().GetGeneration(),
	})
	adoption.UpdatePhase()

	return ctrl.Result{}, c.client.Status().Update(ctx, adoption.ClientObject())
}

func (c *GenericAdoptionController) newOperand() genericAdoption {
	obj, err := c.scheme.New(c.gvk)
	if err != nil {
		panic(err)
	}
	switch o := obj.(type) {
	case *coordinationv1alpha1.Adoption:
		return &GenericAdoption{Adoption: *o}
	case *coordinationv1alpha1.ClusterAdoption:
		return &GenericClusterAdoption{ClusterAdoption: *o}
	}
	panic("unsupported gvk")
}

func (c *GenericAdoptionController) handleDeletion(
	ctx context.Context, adoption genericAdoption,
) error {
	return controllers.HandleCommonDeletion(ctx, adoption.ClientObject(), c.client, c.dw, coordination.CacheFinalizer)
}

// ensures the cache finalizer is set on the given object
func (c *GenericAdoptionController) ensureCacheFinalizer(
	ctx context.Context, adoption genericAdoption,
) error {
	return controllers.EnsureCommonFinalizer(ctx, adoption.ClientObject(), c.client, coordination.CacheFinalizer)
}

// ensures the cache is watching the targetAPI
func (c *GenericAdoptionController) ensureWatch(
	ctx context.Context, adoption genericAdoption,
) error {
	gvk, objType, _ := coordination.UnstructuredFromTargetAPI(
		adoption.GetTargetAPI())

	if err := c.dw.Watch(adoption.ClientObject(), objType); err != nil {
		return fmt.Errorf("watching %s: %w", gvk, err)
	}
	return nil
}
