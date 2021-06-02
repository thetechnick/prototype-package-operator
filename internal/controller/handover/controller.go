package handover

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
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
	"sigs.k8s.io/controller-runtime/pkg/handler"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	"github.com/thetechnick/package-operator/internal/dynamicwatcher"
)

type HandoverReconciler struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	DynamicClient   dynamic.Interface
	DiscoveryClient *discovery.DiscoveryClient

	dw *dynamicwatcher.DynamicWatcher
}

const (
	cacheFinalizer = "coordination.thetechnick.ninja/cache"
)

func (r *HandoverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.dw = dynamicwatcher.New(r.Log, r.Scheme, r.RESTMapper(), r.DynamicClient)

	return ctrl.NewControllerManagedBy(mgr).
		For(&coordinationv1alpha1.Handover{}).
		Watches(r.dw, &handler.EnqueueRequestForOwner{
			OwnerType:    &coordinationv1alpha1.Handover{},
			IsController: false,
		}).
		Complete(r)
}

func (r *HandoverReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	handover := &coordinationv1alpha1.Handover{}
	if err := r.Get(ctx, req.NamespacedName, handover); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !handover.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.handleDeletion(ctx, handover)
	}

	// Add finalizers
	if !controllerutil.ContainsFinalizer(
		handover, cacheFinalizer) {
		controllerutil.AddFinalizer(handover, cacheFinalizer)
		if err := r.Update(ctx, handover); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer: %w", err)
		}
	}

	// Ensure watch
	gvk := schema.GroupVersionKind{
		Group:   handover.Spec.Target.Group,
		Version: handover.Spec.Target.Version,
		Kind:    handover.Spec.Target.Kind,
	}
	objType := &unstructured.Unstructured{}
	objType.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   handover.Spec.Target.Group,
		Version: handover.Spec.Target.Version,
		Kind:    handover.Spec.Target.Kind,
	})
	if err := r.dw.Watch(handover, objType); err != nil {
		return ctrl.Result{}, fmt.Errorf("watching %s: %w", gvk, err)
	}

	// List all objects

	// select all objects with new or old label value
	requirement, err := labels.NewRequirement(
		handover.Spec.Strategy.Relabel.Label,
		selection.In,
		[]string{
			handover.Spec.Strategy.Relabel.NewValue,
			handover.Spec.Strategy.Relabel.OldValue,
		},
	)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building selector: %w", err)
	}
	selector := labels.NewSelector().Add(*requirement)

	objList := &unstructured.UnstructuredList{}
	objList.SetGroupVersionKind(gvk)
	objList.SetKind(gvk.Kind + "List")
	if err := r.List(
		ctx, objList,
		client.InNamespace(handover.Namespace),
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing %s: %w", gvk, err)
	}

	// split into old and new
	groups := groupByLabelValues(
		objList.Items, handover.Spec.Strategy.Relabel.Label,
		handover.Spec.Strategy.Relabel.NewValue,
		handover.Spec.Strategy.Relabel.OldValue,
	)
	newObjs := groups[0]
	oldObjs := groups[1]

	// report counts
	handover.Status.Found = int32(len(objList.Items))
	handover.Status.Updated = int32(len(newObjs))
	handover.Status.Available = int32(countAvailable(objList.Items))

	return ctrl.Result{}, nil
}

func countAvailable(in []unstructured.Unstructured) int {
	return len(in)
}

func groupByLabelValues(in []unstructured.Unstructured, labelKey string, values ...string) [][]unstructured.Unstructured {
	out := make([][]unstructured.Unstructured, len(values))
	for _, obj := range in {
		if obj.GetLabels() == nil {
			continue
		}
		if len(obj.GetLabels()[labelKey]) == 0 {
			continue
		}

		for i, v := range values {
			if obj.GetLabels()[labelKey] == v {
				out[i] = append(out[i], obj)
			}
		}
	}
	return out
}

// handle deletion of the PackageSet
func (r *HandoverReconciler) handleDeletion(
	ctx context.Context,
	handover *coordinationv1alpha1.Handover,
) error {
	if controllerutil.ContainsFinalizer(
		handover, cacheFinalizer) {
		controllerutil.RemoveFinalizer(
			handover, cacheFinalizer)

		if err := r.Update(ctx, handover); err != nil {
			return fmt.Errorf("removing finalizer: %w", err)
		}
	}

	if err := r.dw.Free(handover); err != nil {
		return fmt.Errorf("free cache: %w", err)
	}
	return nil
}
