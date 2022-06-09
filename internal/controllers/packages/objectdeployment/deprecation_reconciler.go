package objectdeployment

import (
	"context"
	"fmt"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultRevisionLimit = 5

// handles deprecation of outdated ObjectSets.
type DeprecationReconciler struct {
	client client.Client
}

func (r *DeprecationReconciler) Reconcile(
	ctx context.Context, objectDeployment genericObjectDeployment,
	currentObjectSet genericObjectSet,
	outdatedObjectSets []genericObjectSet,
) (ctrl.Result, error) {
	var (
		objectSetsForCleanup      []genericObjectSet
		objectDeploymentAvailable bool
	)

	if currentObjectSet != nil &&
		meta.IsStatusConditionTrue(
			currentObjectSet.GetConditions(),
			packagesv1alpha1.PackageSetAvailable,
		) {
		// all old PackageSets are ready for cleanup,
		// because we progressed to a newer version.
		objectSetsForCleanup = outdatedObjectSets
		objectDeploymentAvailable = true

		// We are also no longer progressing, because the latest version is available
		meta.SetStatusCondition(objectDeployment.GetConditions(), metav1.Condition{
			Type:               packagesv1alpha1.ClusterPackageDeploymentProgressing,
			Status:             metav1.ConditionFalse,
			Reason:             "Idle",
			Message:            "Update concluded.",
			ObservedGeneration: objectDeployment.ClientObject().GetGeneration(),
		})
	} else {
		// The latest PackageSet is not Available,
		// but that's Ok, if an earlier one is still up and running.
		for _, outdatedObjectSet := range outdatedObjectSets {
			availableCond := meta.FindStatusCondition(
				outdatedObjectSet.GetConditions(),
				packagesv1alpha1.PackageSetAvailable,
			)
			if availableCond != nil &&
				availableCond.Status == metav1.ConditionTrue &&
				availableCond.ObservedGeneration ==
					outdatedObjectSet.ClientObject().GetGeneration() {
				// Alright! \o/
				// we found an older revision still running
				objectDeploymentAvailable = true
				continue
			}

			// Everything else goes onto the garbage pile for cleanup
			objectSetsForCleanup = append(
				objectSetsForCleanup, outdatedObjectSet)
		}

		// This also means that we are progressing to a new PackageSet,
		// so better report that
		meta.SetStatusCondition(objectDeployment.GetConditions(), metav1.Condition{
			Type:               packagesv1alpha1.PackageDeploymentProgressing,
			Status:             metav1.ConditionTrue,
			Reason:             "Progressing",
			Message:            "Progressing to a new ObjectSet.",
			ObservedGeneration: objectDeployment.ClientObject().GetGeneration(),
		})
	}

	if objectDeploymentAvailable {
		if err := r.deleteObjectSetsOverLimit(
			ctx, objectDeployment, objectSetsForCleanup); err != nil {
			return ctrl.Result{},
				fmt.Errorf("cleaning up outdated ObjectSets: %w", err)
		}

		meta.SetStatusCondition(objectDeployment.GetConditions(), metav1.Condition{
			Type:               packagesv1alpha1.ClusterPackageDeploymentAvailable,
			Status:             metav1.ConditionTrue,
			Reason:             "Available",
			Message:            "At least one revision ObjectSet is Available.",
			ObservedGeneration: objectDeployment.ClientObject().GetGeneration(),
		})
		return ctrl.Result{}, nil
	}

	meta.SetStatusCondition(objectDeployment.GetConditions(), metav1.Condition{
		Type:               packagesv1alpha1.ClusterPackageSetAvailable,
		Status:             metav1.ConditionFalse,
		Reason:             "PackageSetUnready",
		Message:            "Latest PackageSet is not available.",
		ObservedGeneration: objectDeployment.ClientObject().GetGeneration(),
	})

	return ctrl.Result{}, nil
}

func (r *DeprecationReconciler) deleteObjectSetsOverLimit(
	ctx context.Context, objectDeployment genericObjectDeployment,
	objectSetsForCleanup []genericObjectSet,
) error {
	revisionLimit := defaultRevisionLimit
	deploymentRevisionLimit := objectDeployment.GetRevisionHistoryLimit()
	if deploymentRevisionLimit != nil {
		revisionLimit = *deploymentRevisionLimit
	}

	outdatedObjectSetsToDelete := len(objectSetsForCleanup) - revisionLimit

	var outdatedObjectSetsDeleted int
	for _, outdatedObjectSet := range objectSetsForCleanup {
		if outdatedObjectSetsToDelete > outdatedObjectSetsDeleted {
			if err := r.client.Delete(
				ctx, outdatedObjectSet.ClientObject()); err != nil {
				return fmt.Errorf("delete outdated ObjectSet: %w", err)
			}
			outdatedObjectSetsDeleted++
			continue
		}

		// Archive everything that is not fit for deletion
		outdatedObjectSet.SetArchived()
		if err := r.client.Update(
			ctx, outdatedObjectSet.ClientObject()); err != nil {
			return fmt.Errorf("archiving old ObjectSet: %w", err)
		}
	}

	return nil
}
