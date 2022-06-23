package objectdeployments

import (
	"context"
	"fmt"
	"strconv"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// creates a new ObjectSet, if required.
// handles hash collisions if they occur.
type NewRevisionReconciler struct {
	client       client.Client
	scheme       *runtime.Scheme
	newObjectSet func() genericObjectSet
}

func (r *NewRevisionReconciler) Reconcile(
	ctx context.Context, objectDeployment genericObjectDeployment,
	currentObjectSet genericObjectSet,
	outdatedObjectSets []genericObjectSet,
) (ctrl.Result, error) {
	if currentObjectSet != nil {
		// there is a current ObjectSet,
		// no need to create a new one
		return ctrl.Result{}, nil
	}

	log := controllers.LoggerFromContext(ctx)
	log.Info("no current revision")

	latestRevision, err := latestRevision(outdatedObjectSets)
	if err != nil {
		return ctrl.Result{},
			fmt.Errorf("calculating latest revision: %w", err)
	}

	newObjectSet, err := r.newObjectSetFromDeployment(
		objectDeployment, latestRevision)
	if err != nil {
		return ctrl.Result{},
			fmt.Errorf("creating new ObjectSet: %w", err)
	}

	err = r.client.Create(ctx, newObjectSet.ClientObject())
	if err != nil && !errors.IsAlreadyExists(err) {
		return ctrl.Result{}, fmt.Errorf("creating new ObjectSet: %w", err)
	}
	if err == nil {
		return ctrl.Result{}, nil
	}

	// errors.IsAlreadyExists(err)
	conflictingObjectSet := r.newObjectSet()
	if err := r.client.Get(
		ctx, client.ObjectKeyFromObject(newObjectSet.ClientObject()),
		conflictingObjectSet.ClientObject()); err != nil {
		return ctrl.Result{}, fmt.Errorf("getting conflicting ObjectSet: %w", err)
	}

	// sanity check, before we increment the collision counter
	conflictAnnotations := conflictingObjectSet.ClientObject().GetAnnotations()
	if conflictAnnotations != nil && conflictAnnotations[objectSetHashAnnotation] == objectDeployment.GetStatusTemplateHash() {
		log.Info("SANITY CHECK FAILED: no current revision, but hash collision")
		return ctrl.Result{}, nil
	}

	isOwner, err := isOwnerOf(
		objectDeployment.ClientObject(),
		conflictingObjectSet.ClientObject(), r.scheme)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("check owner on conflicting ObjectSet: %w", err)
	}

	if !isOwner ||
		meta.IsStatusConditionTrue(
			conflictingObjectSet.GetConditions(),
			packagesv1alpha1.ObjectSetArchived,
		) {
		// Hash-Collision!
		cc := objectDeployment.GetStatusCollisionCount()
		if cc == nil {
			cc = new(int32)
		}
		*cc++
		objectDeployment.SetStatusCollisionCount(cc)

		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

func (r *NewRevisionReconciler) newObjectSetFromDeployment(
	objectDeployment genericObjectDeployment,
	latestRevision int,
) (genericObjectSet, error) {
	deploy := objectDeployment.ClientObject()

	templateHash := objectDeployment.GetStatusTemplateHash()
	newObjectSet := r.newObjectSet()
	new := newObjectSet.ClientObject()
	new.SetName(deploy.GetName() + "-" + templateHash)
	new.SetNamespace(deploy.GetNamespace())
	new.SetAnnotations(deploy.GetAnnotations())
	new.SetLabels(objectDeployment.GetObjectSetTemplate().Metadata.Labels)
	newObjectSet.SetTemplateSpec(
		objectDeployment.GetObjectSetTemplate().Spec)

	if new.GetAnnotations() == nil {
		new.SetAnnotations(map[string]string{})
	}
	new.GetAnnotations()[objectSetHashAnnotation] = templateHash
	new.GetAnnotations()[objectSetRevisionAnnotation] = strconv.Itoa(latestRevision + 1)
	if err := controllerutil.SetControllerReference(
		deploy, new, r.scheme); err != nil {
		return nil, err
	}
	return newObjectSet, nil
}

// returns the latest revision among the given ObjectSet.
// expects the input objectSet list to be already sorted by revision.
func latestRevision(objectSets []genericObjectSet) (int, error) {
	if len(objectSets) == 0 {
		return 0, nil
	}
	latestObjectSet := objectSets[len(objectSets)-1]
	annotation := latestObjectSet.ClientObject().GetAnnotations()[objectSetRevisionAnnotation]
	if len(annotation) == 0 {
		return 0, nil
	}
	return strconv.Atoi(annotation)
}

func isOwnerOf(owner, obj client.Object, scheme *runtime.Scheme) (bool, error) {
	ownerGVK, err := apiutil.GVKForObject(owner, scheme)
	if err != nil {
		return false, err
	}
	for _, ownerRef := range obj.GetOwnerReferences() {
		if ownerRef.Kind == ownerGVK.Kind &&
			ownerRef.APIVersion == ownerGVK.Group &&
			ownerRef.Name == owner.GetName() &&
			ownerRef.Controller != nil &&
			*ownerRef.Controller {
			return true, nil
		}
	}
	return false, nil
}
