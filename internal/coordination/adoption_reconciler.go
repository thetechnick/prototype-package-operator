package coordination

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
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

func Test() {
	r := NewAdoptionReconciler(coordinationv1alpha1.ClusterAdoption{}, nil, nil, nil, nil, nil)
	r.Reconcile(nil, ctrl.Request{})
}

// Generic reconciler for both Adoption and ClusterAdoption objects.
type GenericAdoptionReconciler[T operandPtr[O], O operand] struct {
	client          client.Client
	log             logr.Logger
	scheme          *runtime.Scheme
	dynamicClient   dynamic.Interface
	discoveryClient *discovery.DiscoveryClient

	dw *dynamicwatcher.DynamicWatcher
}

func NewAdoptionReconciler[T operandPtr[O], O operand](
	o O,
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericAdoptionReconciler[T, O] {
	return &GenericAdoptionReconciler[T, O]{
		client:          c,
		log:             log,
		scheme:          scheme,
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
	}
}

func (r *GenericAdoptionReconciler[T, O]) SetupWithManager(
	mgr ctrl.Manager) error {
	r.dw = dynamicwatcher.New(
		r.log, r.scheme, r.client.RESTMapper(), r.dynamicClient)

	return ctrl.NewControllerManagedBy(mgr).
		For(&coordinationv1alpha1.Adoption{}).
		Watches(r.dw, &dynamicwatcher.EnqueueWatchingObjects{
			WatcherType:      &coordinationv1alpha1.Adoption{},
			WatcherRefGetter: r.dw,
		}).
		Complete(r)
}

func (r *GenericAdoptionReconciler[T, O]) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	adoption := r.newOperand()
	if err := r.client.Get(
		ctx, req.NamespacedName, adoption); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !adoption.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, r.handleDeletion(ctx, adoption)
	}

	if err := r.ensureCacheFinalizer(ctx, adoption); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureWatch(ctx, adoption); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureRelabel(ctx, adoption); err != nil {
		return ctrl.Result{}, err
	}

	setStatus(adoption)

	return ctrl.Result{}, r.client.Status().Update(ctx, adoption)
}

func (r *GenericAdoptionReconciler[T, O]) newOperand() T {
	var o O
	return T(&o)
}

func (r *GenericAdoptionReconciler[T, O]) handleDeletion(
	ctx context.Context, adoption T,
) error {
	if controllerutil.ContainsFinalizer(adoption, cacheFinalizer) {
		controllerutil.RemoveFinalizer(adoption, cacheFinalizer)

		if err := r.client.Update(ctx, adoption); err != nil {
			return fmt.Errorf("removing finalizer: %w", err)
		}
	}

	if err := r.dw.Free(adoption); err != nil {
		return fmt.Errorf("free cache: %w", err)
	}
	return nil
}

// ensures the cache finalizer is set on the given object
func (r *GenericAdoptionReconciler[T, O]) ensureCacheFinalizer(
	ctx context.Context, adoption T) error {
	if !controllerutil.ContainsFinalizer(
		adoption, cacheFinalizer) {
		controllerutil.AddFinalizer(adoption, cacheFinalizer)
		if err := r.client.Update(ctx, adoption); err != nil {
			return fmt.Errorf("adding finalizer: %w", err)
		}
	}
	return nil
}

// ensures the cache is watching the targetAPI
func (r *GenericAdoptionReconciler[T, O]) ensureWatch(
	ctx context.Context, adoption T,
) error {
	gvk, objType, _ := unstructuredFromTargetAPI(getTargetAPI(adoption))

	if err := r.dw.Watch(adoption, objType); err != nil {
		return fmt.Errorf("watching %s: %w", gvk, err)
	}
	return nil
}

func (r *GenericAdoptionReconciler[T, O]) ensureRelabel(
	ctx context.Context, adoption T,
) error {
	specLabels := getSpecLabels(adoption)

	selector, err := negativeLabelSelectorFromLabels(specLabels)
	if err != nil {
		return err
	}

	gvk, _, objListType := unstructuredFromTargetAPI(getTargetAPI(adoption))

	// List all the things!
	if err := r.client.List(
		ctx, objListType,
		client.InNamespace(adoption.GetNamespace()), // can also set this for ClusterAdoption without issue.
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	); err != nil {
		return fmt.Errorf("listing %s: %w", gvk, err)
	}

	for _, obj := range objListType.Items {
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		for k, v := range specLabels {
			labels[k] = v
		}
		obj.SetLabels(labels)

		if err := r.client.Update(ctx, &obj); err != nil {
			return fmt.Errorf("setting labels: %w", err)
		}
	}
	return nil
}

func negativeLabelSelectorFromLabels(specLabels map[string]string) (labels.Selector, error) {
	// Build selector
	var requirements []labels.Requirement
	for k := range specLabels {
		requirement, err := labels.NewRequirement(
			k, selection.DoesNotExist, nil)
		if err != nil {
			return nil, fmt.Errorf("building selector: %w", err)
		}
		requirements = append(requirements, *requirement)
	}
	selector := labels.NewSelector().Add(requirements...)
	return selector, nil
}

func setStatus(adoption client.Object) {
	var conds *[]metav1.Condition
	switch o := adoption.(type) {
	case *coordinationv1alpha1.Adoption:
		o.Status.ObservedGeneration = o.Generation
		o.Status.Phase = coordinationv1alpha1.AdoptionPhaseActive

	case *coordinationv1alpha1.ClusterAdoption:
		o.Status.ObservedGeneration = o.Generation
		o.Status.Phase = coordinationv1alpha1.ClusterAdoptionPhaseActive
	}

	meta.SetStatusCondition(conds, metav1.Condition{
		Type:               coordinationv1alpha1.AdoptionActive,
		Status:             metav1.ConditionTrue,
		Reason:             "Setup",
		Message:            "Controller is setup and adding labels.",
		ObservedGeneration: adoption.GetGeneration(),
	})

}

func getSpecLabels(adoption client.Object) map[string]string {
	switch o := adoption.(type) {
	case *coordinationv1alpha1.Adoption:
		return o.Spec.Strategy.Static.Labels
	case *coordinationv1alpha1.ClusterAdoption:
		return o.Spec.Strategy.Static.Labels
	}
	return nil
}

func getTargetAPI(adoption client.Object) coordinationv1alpha1.TargetAPI {
	switch o := adoption.(type) {
	case *coordinationv1alpha1.Adoption:
		return o.Spec.TargetAPI
	case *coordinationv1alpha1.ClusterAdoption:
		return o.Spec.TargetAPI
	}
	return coordinationv1alpha1.TargetAPI{}
}

// builds unstrucutred objects from a TargetAPI object.
func unstructuredFromTargetAPI(targetAPI coordinationv1alpha1.TargetAPI) (
	gvk schema.GroupVersionKind,
	objType *unstructured.Unstructured,
	objListType *unstructured.UnstructuredList,
) {
	gvk = schema.GroupVersionKind{
		Group:   targetAPI.Group,
		Version: targetAPI.Version,
		Kind:    targetAPI.Kind,
	}

	objType = &unstructured.Unstructured{}
	objType.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   targetAPI.Group,
		Version: targetAPI.Version,
		Kind:    targetAPI.Kind,
	})

	objListType = &unstructured.UnstructuredList{}
	objListType.SetGroupVersionKind(gvk)
	objListType.SetKind(gvk.Kind + "List")
	return
}
