package controller

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
)

type PackageDeploymentReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *PackageDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&packagesv1alpha1.PackageDeployment{}).
		Owns(&packagesv1alpha1.PackageSet{}).
		Complete(r)
}

const (
	packageSetHashAnnotation     = "packages.thetechnick.ninja/hash"
	packageSetRevisionAnnotation = "packages.thetechnick.ninja/revision"
)

func (r *PackageDeploymentReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("PackageDeployment", req.NamespacedName.String())

	packageDeployment := &packagesv1alpha1.PackageDeployment{}
	if err := r.Get(ctx, req.NamespacedName, packageDeployment); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	packageSets, err := r.listPackageSetsByRevision(ctx, packageDeployment)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("listing PackageSets by revision: %w", err)
	}

	latestRevision, err := latestRevision(packageSets)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("calculating latest revision: %w", err)
	}

	templateHash := computeHash(
		&packageDeployment.Spec.Template,
		packageDeployment.Status.CollisionCount)

	newPackageSet := &packagesv1alpha1.PackageSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        packageDeployment.Name + "-" + templateHash,
			Namespace:   packageDeployment.Namespace,
			Annotations: packageDeployment.Annotations,
			Labels:      packageDeployment.Spec.Template.Metadata.Labels,
		},
		Spec: packagesv1alpha1.PackageSetSpec{
			PackageSetTemplateSpec: packageDeployment.Spec.Template.Spec,
		},
	}
	if newPackageSet.Annotations == nil {
		newPackageSet.Annotations = map[string]string{}
	}
	newPackageSet.Annotations[packageSetHashAnnotation] = templateHash
	newPackageSet.Annotations[packageSetRevisionAnnotation] = strconv.Itoa(latestRevision + 1)

	if err := controllerutil.SetControllerReference(
		packageDeployment, newPackageSet, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// What's the current PackageSet revision to check, or do we have to create it?
	var (
		currentPackageSet   *packagesv1alpha1.PackageSet
		outdatedPackageSets []packagesv1alpha1.PackageSet
	)
	for i := range packageSets {
		if equality.Semantic.DeepEqual(newPackageSet.Spec.PackageSetTemplateSpec, packageSets[i].Spec.PackageSetTemplateSpec) &&
			!meta.IsStatusConditionTrue(packageSets[i].Status.Conditions, packagesv1alpha1.PackageSetArchived) {
			currentPackageSet = packageSets[i].DeepCopy()
			continue
		}
		outdatedPackageSets = append(outdatedPackageSets, packageSets[i])
	}

	// Ensure Objects that are a part of the current PackageSet are not reconciled by other PackageSets.
	pausedObjects, err := pausedObjectsFromPackageSet(newPackageSet)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting paused objects from PackageSet: %w", err)
	}

	var outdatedPackageSetsDeleted int
	for _, outdatedPackageSet := range outdatedPackageSets {
		if meta.IsStatusConditionTrue(
			outdatedPackageSet.Status.Conditions,
			packagesv1alpha1.PackageSetArchived,
		) {
			// This PackageSet is archived and should stay that way.
			continue
		}

		if !equality.Semantic.DeepEqual(pausedObjects, outdatedPackageSet.Spec.PausedFor) {
			outdatedPackageSet.Spec.PausedFor = pausedObjects
			if err := r.Update(ctx, &outdatedPackageSet); err != nil {
				return ctrl.Result{}, fmt.Errorf("updating outdated PackageSet: %w", err)
			}
		}

		if outdatedPackageSet.Generation != outdatedPackageSet.Status.ObservedGeneration &&
			!equality.Semantic.DeepDerivative(pausedObjects, outdatedPackageSet.Status.PausedFor) {
			log.Info(
				"waiting for outdated PackageSet to be paused",
				"PackageSet", client.ObjectKeyFromObject(&outdatedPackageSet).String())
			// we can return here, because a status update to the PackageSet will reenqueue this PackageDeployment
			return ctrl.Result{}, nil
		}
	}

	// Create new PackageSet to progress to the next version.
	if currentPackageSet == nil {
		packageDeployment.Status.Phase = packagesv1alpha1.PackageDeploymentProgressing
		packageDeployment.Status.ObservedGeneration = packageDeployment.Generation
		if err := r.Status().Update(ctx, packageDeployment); err != nil {
			return ctrl.Result{}, err
		}

		err := r.Create(ctx, newPackageSet)
		if errors.IsAlreadyExists(err) {
			conflictingPackageSet := &packagesv1alpha1.PackageSet{}
			if err := r.Get(ctx, client.ObjectKeyFromObject(newPackageSet), conflictingPackageSet); err != nil {
				return ctrl.Result{}, fmt.Errorf("getting conflicting PackageSet: %w", err)
			}

			isOwner, err := isOwnerOf(packageDeployment, conflictingPackageSet, r.Scheme)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("check owner on conflicting PackageSet: %w", err)
			}
			if isOwner &&
				equality.Semantic.DeepEqual(newPackageSet.Spec.PackageSetTemplateSpec, conflictingPackageSet.Spec.PackageSetTemplateSpec) &&
				!meta.IsStatusConditionTrue(conflictingPackageSet.Status.Conditions, packagesv1alpha1.PackageSetArchived) {
				// Hey! This looks like what we wanted to create anyway.
				// Looks like a slow cache.
				currentPackageSet = conflictingPackageSet
			} else {
				// Looks like a collision, retry
				if packageDeployment.Status.CollisionCount == nil {
					packageDeployment.Status.CollisionCount = new(int32)
				}
				*packageDeployment.Status.CollisionCount++
				return ctrl.Result{Requeue: true}, r.Status().Update(ctx, packageDeployment)
			}
		} else if err != nil {
			return ctrl.Result{}, fmt.Errorf("creating new PackageSet: %w", err)
		} else {
			currentPackageSet = newPackageSet
		}
	}

	var packageDeploymentAvailable bool
	var packageSetsForCleanup []packagesv1alpha1.PackageSet
	if meta.IsStatusConditionTrue(
		currentPackageSet.Status.Conditions, packagesv1alpha1.PackageSetAvailable) &&
		currentPackageSet.Generation == currentPackageSet.Status.ObservedGeneration {

		// all old PackageSets are ready for cleanup,
		// because we progressed to a newer version.
		packageSetsForCleanup = outdatedPackageSets

		packageDeploymentAvailable = true

		// We are also no longer progressing, because the latest version is available
		meta.SetStatusCondition(&packageDeployment.Status.Conditions, metav1.Condition{
			Type:               packagesv1alpha1.PackageDeploymentProgressing,
			Status:             metav1.ConditionFalse,
			Reason:             "Idle",
			Message:            "Update concluded.",
			ObservedGeneration: packageDeployment.Generation,
		})
		packageDeployment.Status.Phase = packagesv1alpha1.PackageDeploymentAvailable
	} else {
		// The latest PackageSet is not Available, but that's Ok, if an earlier one is still up and running.
		for _, outdatedPackageSet := range outdatedPackageSets {
			if meta.IsStatusConditionTrue(
				outdatedPackageSet.Status.Conditions, packagesv1alpha1.PackageSetAvailable) &&
				outdatedPackageSet.Generation == outdatedPackageSet.Status.ObservedGeneration {
				// Alright! \o/
				packageDeploymentAvailable = true
				continue
			}

			// Everything else goes onto the garbage pile for cleanup
			packageSetsForCleanup = append(packageSetsForCleanup, outdatedPackageSet)
		}

		// This also means that we are progressing to a new PackageSet, so better report that
		meta.SetStatusCondition(&packageDeployment.Status.Conditions, metav1.Condition{
			Type:               packagesv1alpha1.PackageDeploymentProgressing,
			Status:             metav1.ConditionTrue,
			Reason:             "Progressing",
			Message:            "Progressing to a new PackageSet.",
			ObservedGeneration: packageDeployment.Generation,
		})
		packageDeployment.Status.Phase = packagesv1alpha1.PackageDeploymentPhaseProgressing
	}

	if packageDeploymentAvailable {
		// Delete PackageSets over limit
		revisionLimit := 5
		if packageDeployment.Spec.RevisionHistoryLimit != nil {
			revisionLimit = *packageDeployment.Spec.RevisionHistoryLimit
		}
		outdatedPackagSetsToDelete := len(outdatedPackageSets) - revisionLimit

		// Some PackageSet is up and ready, so we can cleanup old stuff.
		for _, outdatedPackageSet := range packageSetsForCleanup {
			if outdatedPackagSetsToDelete > outdatedPackageSetsDeleted {
				if err := r.Delete(ctx, &outdatedPackageSet); err != nil {
					return ctrl.Result{}, fmt.Errorf("delete outdated PackageSet: %w", err)
				}
				outdatedPackageSetsDeleted++
				continue
			}

			outdatedPackageSet.Spec.Archived = true
			if err := r.Update(ctx, &outdatedPackageSet); err != nil {
				return ctrl.Result{}, fmt.Errorf("archiving old PackageSet: %w", err)
			}
		}

		meta.SetStatusCondition(&packageDeployment.Status.Conditions, metav1.Condition{
			Type:               packagesv1alpha1.PackageDeploymentAvailable,
			Status:             metav1.ConditionTrue,
			Reason:             "Available",
			Message:            "At least one revision PackageSet is Available.",
			ObservedGeneration: packageDeployment.Generation,
		})
		packageDeployment.Status.ObservedGeneration = packageDeployment.Generation
		if err := r.Status().Update(ctx, packageDeployment); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	meta.SetStatusCondition(&packageDeployment.Status.Conditions, metav1.Condition{
		Type:               packagesv1alpha1.PackageSetAvailable,
		Status:             metav1.ConditionFalse,
		Reason:             "PackageSetUnready",
		Message:            "Latest PackageSet is not available.",
		ObservedGeneration: packageDeployment.Generation,
	})
	packageDeployment.Status.Phase = packagesv1alpha1.PackageDeploymentPhaseNotReady
	packageDeployment.Status.ObservedGeneration = packageDeployment.Generation
	return ctrl.Result{}, r.Status().Update(ctx, packageDeployment)
}

type packageSetsByRevision []packagesv1alpha1.PackageSet

func (a packageSetsByRevision) Len() int      { return len(a) }
func (a packageSetsByRevision) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a packageSetsByRevision) Less(i, j int) bool {
	if a[i].Annotations == nil ||
		len(a[i].Annotations[packageSetRevisionAnnotation]) == 0 ||
		a[j].Annotations == nil ||
		len(a[j].Annotations[packageSetRevisionAnnotation]) == 0 {
		return a[i].CreationTimestamp.Before(&a[j].CreationTimestamp)
	}

	psIRevision, _ := strconv.Atoi(a[i].Annotations[packageSetRevisionAnnotation])
	psJRevision, _ := strconv.Atoi(a[j].Annotations[packageSetRevisionAnnotation])

	return psIRevision < psJRevision
}

func (r *PackageDeploymentReconciler) listPackageSetsByRevision(
	ctx context.Context,
	packageDeployment *packagesv1alpha1.PackageDeployment,
) ([]packagesv1alpha1.PackageSet, error) {
	packageSetSelector, err := metav1.LabelSelectorAsSelector(
		&packageDeployment.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("invalid selector: %w", err)
	}

	packageSetList := &packagesv1alpha1.PackageSetList{}
	if err := r.List(
		ctx, packageSetList,
		client.MatchingLabelsSelector{
			Selector: packageSetSelector,
		},
		client.InNamespace(packageDeployment.Namespace),
	); err != nil {
		return nil, fmt.Errorf("listing PackageSets: %w", err)
	}

	sort.Sort(packageSetsByRevision(packageSetList.Items))
	return packageSetList.Items, nil
}
