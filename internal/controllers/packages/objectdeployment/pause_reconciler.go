package objectdeployment

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EnsurePauseReconciler handles pausing of existing ObjectSets,
// to prevent collisions and double owned objects.
type EnsurePauseReconciler struct {
	client                      client.Client
	listObjectSetsForDeployment listObjectSetsForDeploymentFn
	reconcilers                 []objectSetReconciler
}

type objectSetReconciler interface {
	Reconcile(
		ctx context.Context, objectDeployment genericObjectDeployment,
		currentObjectSet genericObjectSet,
		outdatedObjectSet []genericObjectSet,
	) (ctrl.Result, error)
}

type listObjectSetsForDeploymentFn func(
	ctx context.Context, objectDeployment genericObjectDeployment,
) ([]genericObjectSet, error)

func (r *EnsurePauseReconciler) Reconcile(
	ctx context.Context, objectDeployment genericObjectDeployment,
) (ctrl.Result, error) {
	log := controllers.LoggerFromContext(ctx)

	pausedObjects, err := pausedObjectsFromPhases(
		objectDeployment.GetPackageSetTemplate().Spec.Phases)
	if err != nil {
		return ctrl.Result{},
			fmt.Errorf("getting paused objects from ObjectSet: %w", err)
	}

	objectSets, err := r.listObjectSetsForDeployment(ctx, objectDeployment)
	if err != nil {
		return ctrl.Result{},
			fmt.Errorf("list ObjectSets: %w", err)
	}
	// Ensure everything is sorted by revision.
	sort.Sort(objectSetsByRevision(objectSets))

	var (
		currentObjectSet   genericObjectSet
		outdatedObjectSets []genericObjectSet
	)
	for _, objectSet := range objectSets {
		annotations := objectSet.ClientObject().GetAnnotations()
		if annotations == nil {
			// TODO: Raise error,
			// no PackageSet should be missing this annotation.
			// -> ensure in HashReconciler?
			continue
		}
		if annotations[objectSetHashAnnotation] ==
			objectDeployment.GetStatusTemplateHash() {
			// This ObjectSet is up-to-date, we don't touch this.
			currentObjectSet = objectSet
			continue
		}
		outdatedObjectSets = append(outdatedObjectSets, objectSet)

		if meta.IsStatusConditionTrue(
			objectSet.GetConditions(), packagesv1alpha1.PackageSetArchived) {
			// already archived, no one cares
			continue
		}

		if !equality.Semantic.DeepEqual(
			pausedObjects, objectSet.GetSpecPausedFor()) {
			objectSet.SetSpecPausedFor(pausedObjects)
			if err := r.client.Update(
				ctx, objectSet.ClientObject()); err != nil {
				return ctrl.Result{},
					fmt.Errorf("updating outdated ObjectSet: %w", err)
			}
		}

		// ensure everything we need is paused
		if !equality.Semantic.DeepDerivative(
			pausedObjects, objectSet.GetStatusPausedFor()) {
			log.Info(
				"waiting for outdated ObjectSet to be paused",
				"ObjectSet", client.ObjectKeyFromObject(objectSet.ClientObject()).String())
			// we can return here, because a status update to the PackageSet will reenqueue this PackageDeployment
			return ctrl.Result{}, nil
		}
	}

	var (
		res ctrl.Result
	)
	for _, r := range r.reconcilers {
		res, err = r.Reconcile(
			ctx, objectDeployment, currentObjectSet, outdatedObjectSets)
		if err != nil || !res.IsZero() {
			break
		}
	}
	if err != nil {
		return res, err
	}

	return ctrl.Result{}, nil
}

type objectSetsByRevision []genericObjectSet

func (a objectSetsByRevision) Len() int      { return len(a) }
func (a objectSetsByRevision) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a objectSetsByRevision) Less(i, j int) bool {
	iObj := a[i].ClientObject()
	jObj := a[j].ClientObject()

	if iObj.GetAnnotations() == nil ||
		len(iObj.GetAnnotations()[objectSetRevisionAnnotation]) == 0 ||
		jObj.GetAnnotations() == nil ||
		len(jObj.GetAnnotations()[objectSetRevisionAnnotation]) == 0 {
		return iObj.GetCreationTimestamp().UTC().Before(
			jObj.GetCreationTimestamp().UTC())
	}

	psIRevision, _ := strconv.Atoi(iObj.GetAnnotations()[objectSetRevisionAnnotation])
	psJRevision, _ := strconv.Atoi(jObj.GetAnnotations()[objectSetRevisionAnnotation])

	return psIRevision < psJRevision
}
