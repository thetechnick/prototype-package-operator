package objectdeployment

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	objectSetHashAnnotation     = "packages.thetechnick.ninja/hash"
	objectSetRevisionAnnotation = "packages.thetechnick.ninja/revision"
)

// Generic reconciler for both ObjectDeployment and ClusterObjectDeployment objects.
type GenericObjectDeploymentController struct {
	gvk      schema.GroupVersionKind
	childGVK schema.GroupVersionKind

	client     client.Client
	log        logr.Logger
	scheme     *runtime.Scheme
	reconciler []reconciler
}

type reconciler interface {
	Reconcile(ctx context.Context, objectSet genericObjectDeployment) (ctrl.Result, error)
}

func NewObjectDeploymentController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
) *GenericObjectDeploymentController {
	return NewGenericObjectDeploymentController(
		packagesv1alpha1.GroupVersion.WithKind("ObjectDeployment"),
		packagesv1alpha1.GroupVersion.WithKind("ObjectSet"),
		c, log, scheme,
	)
}

func NewClusterObjectDeploymentController(
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
) *GenericObjectDeploymentController {
	return NewGenericObjectDeploymentController(
		packagesv1alpha1.GroupVersion.WithKind("ClusterObjectDeployment"),
		packagesv1alpha1.GroupVersion.WithKind("ClusterObjectSet"),
		c, log, scheme,
	)
}

func NewGenericObjectDeploymentController(
	gvk schema.GroupVersionKind,
	childGVK schema.GroupVersionKind,
	c client.Client, log logr.Logger, scheme *runtime.Scheme,
) *GenericObjectDeploymentController {
	controller := &GenericObjectDeploymentController{
		gvk:      gvk,
		childGVK: childGVK,

		client: c,
		log:    log,
		scheme: scheme,
	}
	controller.reconciler = []reconciler{
		&HashReconciler{},
		&EnsurePauseReconciler{
			client:                      c,
			listObjectSetsForDeployment: controller.listObjectSetsByRevision,
			reconcilers: []objectSetReconciler{
				&NewRevisionReconciler{
					client:       c,
					scheme:       scheme,
					newObjectSet: controller.newOperandChild,
				},
				&DeprecationReconciler{
					client: c,
				},
			},
		},
	}

	return controller
}

func (c *GenericObjectDeploymentController) newOperand() genericObjectDeployment {
	obj, err := c.scheme.New(c.gvk)
	if err != nil {
		panic(err)
	}

	switch o := obj.(type) {
	case *packagesv1alpha1.ObjectDeployment:
		return &GenericObjectDeployment{ObjectDeployment: *o}
	case *packagesv1alpha1.ClusterObjectDeployment:
		return &GenericClusterObjectDeployment{ClusterObjectDeployment: *o}
	}
	panic("unsupported gvk")
}

func (c *GenericObjectDeploymentController) newOperandChild() genericObjectSet {
	obj, err := c.scheme.New(c.childGVK)
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

func (c *GenericObjectDeploymentController) newOperandChildList() genericObjectSetList {
	childListGVK := c.childGVK.GroupVersion().
		WithKind(c.childGVK.Kind + "List")
	obj, err := c.scheme.New(childListGVK)
	if err != nil {
		panic(err)
	}

	switch o := obj.(type) {
	case *packagesv1alpha1.ObjectSetList:
		return &GenericObjectSetList{ObjectSetList: *o}
	case *packagesv1alpha1.ClusterObjectSetList:
		return &GenericClusterObjectSetList{ClusterObjectSetList: *o}
	}
	panic("unsupported gvk")
}

func (c *GenericObjectDeploymentController) SetupWithManager(
	mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(c.newOperand().ClientObject()).
		Owns(c.newOperandChild().ClientObject()).
		Complete(c)
}

func (c *GenericObjectDeploymentController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("ObjectSet", req.NamespacedName.String())
	ctx = controllers.ContextWithLogger(ctx, log)

	objectDeployment := c.newOperand()
	if err := c.client.Get(
		ctx, req.NamespacedName, objectDeployment.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var (
		res ctrl.Result
		err error
	)
	for _, r := range c.reconciler {
		res, err = r.Reconcile(ctx, objectDeployment)
		if err != nil || !res.IsZero() {
			break
		}
	}
	if err != nil {
		return res, err
	}

	objectDeployment.UpdatePhase()
	return res, c.client.Status().Update(ctx, objectDeployment.ClientObject())
}

func (c *GenericObjectDeploymentController) listObjectSetsByRevision(
	ctx context.Context,
	objectDeployment genericObjectDeployment,
) ([]genericObjectSet, error) {
	labelSelector := objectDeployment.GetSelector()
	objectSetSelector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		return nil, fmt.Errorf("invalid selector: %w", err)
	}

	objectSetList := c.newOperandChildList()
	if err := c.client.List(
		ctx, objectSetList.ClientObjectList(),
		client.MatchingLabelsSelector{
			Selector: objectSetSelector,
		},
		client.InNamespace(objectDeployment.ClientObject().GetNamespace()),
	); err != nil {
		return nil, fmt.Errorf("listing ObjectSets: %w", err)
	}

	items := objectSetList.GetItems()
	return items, nil
}
