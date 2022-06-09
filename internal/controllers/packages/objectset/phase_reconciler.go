package objectset

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers"
	"github.com/thetechnick/package-operator/internal/controllers/packages"
	"github.com/thetechnick/package-operator/internal/dynamicwatcher"
	internalprobe "github.com/thetechnick/package-operator/internal/probe"
)

type ObjectSetPhaseReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	dw     *dynamicwatcher.DynamicWatcher
}

func (r *ObjectSetPhaseReconciler) Reconcile(
	ctx context.Context, objectSet genericObjectSet,
) (ctrl.Result, error) {
	if objectSet.IsArchived() {
		return ctrl.Result{}, nil
	}

	probe := parseProbes(objectSet.GetReadinessProbes())
	phases := objectSet.GetPhases()
	for _, phase := range phases {
		stop, err := r.reconcilePhase(ctx, objectSet, &phase, probe)
		if err != nil {
			return ctrl.Result{}, err
		}
		if stop {
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, nil
}

func (r *ObjectSetPhaseReconciler) reconcilePhase(
	ctx context.Context,
	objectSet genericObjectSet,
	phase *packagesv1alpha1.PackagePhase,
	probe internalprobe.Interface,
) (stop bool, err error) {
	var failedProbes []string

	// Reconcile objects in phase
	for _, phaseObject := range phase.Objects {
		obj, err := unstructuredFromPackageObject(&phaseObject)
		if err != nil {
			return false, err
		}
		if err := r.reconcileObject(ctx, objectSet, obj); err != nil {
			return false, err
		}

		if success, message := probe.Probe(obj); !success {
			gvk := obj.GroupVersionKind()
			failedProbes = append(failedProbes,
				fmt.Sprintf("%s %s %s/%s: %s", gvk.Group, gvk.Kind, obj.GetNamespace(), obj.GetName(), message))
		}
	}

	if len(failedProbes) == 0 {
		if !meta.IsStatusConditionTrue(*objectSet.GetConditions(), packagesv1alpha1.PackageSetSucceeded) {
			meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
				Type:    packagesv1alpha1.PackageSetSucceeded,
				Status:  metav1.ConditionTrue,
				Reason:  "AvailableOnce",
				Message: "Package was available once and passed all probes.",
			})
		}

		meta.RemoveStatusCondition(objectSet.GetConditions(), packagesv1alpha1.PackageSetArchived)
		meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
			Type:               packagesv1alpha1.PackageSetAvailable,
			Status:             metav1.ConditionTrue,
			Reason:             "Available",
			Message:            "Package is available and passes all probes.",
			ObservedGeneration: objectSet.ClientObject().GetGeneration(),
		})

		return false, nil
	}

	meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
		Type:               packagesv1alpha1.PackageSetAvailable,
		Status:             metav1.ConditionFalse,
		Reason:             "ProbeFailure",
		Message:            fmt.Sprintf("Phase %q failed: %s", phase.Name, strings.Join(failedProbes, ", ")),
		ObservedGeneration: objectSet.ClientObject().GetGeneration(),
	})
	return true, nil
}

func (r *ObjectSetPhaseReconciler) reconcileObject(
	ctx context.Context,
	objectSet genericObjectSet,
	obj *unstructured.Unstructured,
) error {
	log := controllers.LoggerFromContext(ctx)

	// Add our own label
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[packages.PackageSetLabelKey] = strings.Replace(client.ObjectKeyFromObject(objectSet.ClientObject()).String(), "/", "-", -1)
	obj.SetLabels(labels)

	// Force namespace to this one
	obj.SetNamespace(objectSet.ClientObject().GetNamespace())

	// Ensure we are owner
	if err := controllerutil.SetControllerReference(objectSet.ClientObject(), obj, r.scheme); err != nil {
		return err
	}

	// Ensure to watchlist
	if err := r.dw.Watch(objectSet.ClientObject(), obj); err != nil {
		return fmt.Errorf("watching new resource: %w", err)
	}

	currentObj := obj.DeepCopy()
	err := r.client.Get(ctx, client.ObjectKeyFromObject(obj), currentObj)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("getting: %w", err)
	}

	if objectSet.IsObjectPaused(obj) {
		// Paused, don't reconcile.
		// Just report the latest object state.
		*obj = *currentObj
		return nil
	}

	if errors.IsNotFound(err) {
		err := r.client.Create(ctx, obj)
		if err != nil {
			return fmt.Errorf("creating: %w", err)
		}
	}

	// Adoption/Handover process
	var isOwner bool
	for _, ownerRef := range currentObj.GetOwnerReferences() {
		isOwner = ownerRef.UID == objectSet.ClientObject().GetUID()
		if isOwner {
			break
		}
	}
	// Let's take over ownership from the other PackageSet.
	var newOwnerRefs []metav1.OwnerReference
	for _, ownerRef := range currentObj.GetOwnerReferences() {
		ownerRef.Controller = nil
		newOwnerRefs = append(newOwnerRefs, ownerRef)
	}
	obj.SetOwnerReferences(newOwnerRefs)

	if !isOwner {
		// Just patch the OwnerReferences of the object,
		// or we by pass the DeepDerivative check and
		// might update other object properties.
		updatedOwnersObj := currentObj.DeepCopy()
		updatedOwnersObj.SetOwnerReferences(newOwnerRefs)
		log.Info("patching for ownership", "obj", client.ObjectKeyFromObject(obj))
		if err := r.client.Patch(
			ctx, currentObj, client.MergeFrom(updatedOwnersObj)); err != nil {
			return fmt.Errorf("patching Owners: %w", err)
		}
	}

	if err := controllerutil.SetControllerReference(
		objectSet.ClientObject(), obj, r.scheme); err != nil {
		return err
	}

	// Update
	if !equality.Semantic.DeepDerivative(obj.Object, currentObj.Object) {
		log.Info("patching spec", "obj", client.ObjectKeyFromObject(obj))
		// this is only updating "known" fields,
		// so annotations/labels and other properties will be preserved.
		err := r.client.Patch(
			ctx, obj, client.MergeFrom(&unstructured.Unstructured{}))

		// Alternative to override the object completely:
		// err := r.Update(ctx, obj)
		if err != nil {
			return fmt.Errorf("patching spec: %w", err)
		}
	} else {
		*obj = *currentObj
	}

	return nil
}

func parseProbes(
	packageProbes []packagesv1alpha1.PackageProbe,
) internalprobe.Interface {
	var probes internalprobe.ProbeList
	for _, packageProbe := range packageProbes {
		probe := internalprobe.Parse(packageProbe.Probes)

		// wrap filter type
		selector := packageProbe.Selector
		switch selector.Type {
		case packagesv1alpha1.ProbeSelectorKind:
			if selector.Kind == nil {
				continue
			}

			probe = &internalprobe.KindSelector{
				Interface: probe,
				GroupKind: schema.GroupKind{
					Group: selector.Kind.Group,
					Kind:  selector.Kind.Kind,
				},
			}

		default:
			// Unknown selector type
			continue
		}

		probes = append(probes, probe)
	}
	return probes
}
