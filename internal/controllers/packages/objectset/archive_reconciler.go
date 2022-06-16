package objectset

import (
	"context"
	"fmt"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type dynamicWatchFreer interface {
	Free(obj client.Object) error
}

type ArchivedObjectSetReconciler struct {
	client client.Client
	dw     dynamicWatchFreer
}

func (r *ArchivedObjectSetReconciler) Reconcile(ctx context.Context, objectSet genericObjectSet) (ctrl.Result, error) {
	if !objectSet.IsArchived() {
		return ctrl.Result{}, nil
	}

	phases := objectSet.GetPhases()

	// Scale to zero
	// TODO: Reverse phases and wait for all objects in a phase to be gone before continuing
	// -> Like deletion handling
	for _, phase := range phases {
		for _, phaseObject := range phase.Objects {
			obj, err := unstructuredFromObjectObject(&phaseObject)
			if err != nil {
				return ctrl.Result{}, err
			}
			obj.SetNamespace(objectSet.ClientObject().GetNamespace())

			if !objectSet.IsObjectPaused(obj) {
				if err := r.client.Delete(ctx, obj); err != nil && !errors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
			}
		}
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
	objectSet.SetStatusPausedFor(objectSet.GetPausedFor())

	if err := r.dw.Free(objectSet.ClientObject()); err != nil {
		return ctrl.Result{}, fmt.Errorf("free cache: %w", err)
	}

	return ctrl.Result{}, nil
}
