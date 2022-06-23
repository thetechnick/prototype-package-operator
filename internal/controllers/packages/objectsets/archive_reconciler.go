package objectsets

import (
	"context"
	"fmt"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type dynamicWatchFreer interface {
	Free(obj client.Object) error
}

type ArchivedObjectSetReconciler struct {
	teardownHandler teardownHandler
	dw              dynamicWatchFreer
}

type teardownHandler interface {
	Teardown(
		ctx context.Context, objectSet genericObjectSet,
	) (done bool, err error)
}

func (r *ArchivedObjectSetReconciler) Reconcile(ctx context.Context, objectSet genericObjectSet) (ctrl.Result, error) {
	if !objectSet.IsArchived() {
		return ctrl.Result{}, nil
	}

	done, err := r.teardownHandler.Teardown(ctx, objectSet)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("archiving ObjectSet: %w", err)
	}
	if !done {
		conditions := objectSet.GetConditions()
		meta.RemoveStatusCondition(conditions, packagesv1alpha1.ObjectSetPaused)
		meta.RemoveStatusCondition(conditions, packagesv1alpha1.ObjectSetAvailable)
		meta.SetStatusCondition(conditions, metav1.Condition{
			Type:               packagesv1alpha1.ObjectSetArchived,
			Status:             metav1.ConditionFalse,
			Reason:             "Archived",
			Message:            "ObjectSet is tearing down.",
			ObservedGeneration: objectSet.ClientObject().GetGeneration(),
		})
	}

	conditions := objectSet.GetConditions()
	meta.RemoveStatusCondition(conditions, packagesv1alpha1.ObjectSetPaused)
	meta.RemoveStatusCondition(conditions, packagesv1alpha1.ObjectSetAvailable)
	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:               packagesv1alpha1.ObjectSetArchived,
		Status:             metav1.ConditionTrue,
		Reason:             "Archived",
		Message:            "ObjectSet is archived.",
		ObservedGeneration: objectSet.ClientObject().GetGeneration(),
	})

	if err := r.dw.Free(objectSet.ClientObject()); err != nil {
		return ctrl.Result{}, fmt.Errorf("free cache: %w", err)
	}

	return ctrl.Result{}, nil
}
