package packages

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers"
	internalprobe "github.com/thetechnick/package-operator/internal/probe"
)

type PhaseReconciler struct {
	dw     dynamicWatcher
	client client.Client
	scheme *runtime.Scheme

	ownerStrategy ownerStrategy
}

func NewPhaseReconciler(
	dw dynamicWatcher,
	c client.Client,
	scheme *runtime.Scheme,
	ownerStrategy ownerStrategy,
) *PhaseReconciler {
	return &PhaseReconciler{
		dw:            dw,
		client:        c,
		scheme:        scheme,
		ownerStrategy: ownerStrategy,
	}
}

type ownerStrategy interface {
	IsOwner(owner, obj metav1.Object) bool
	ReleaseController(obj metav1.Object)
	SetControllerReference(owner, obj metav1.Object, scheme *runtime.Scheme) error
}

type dynamicWatcher interface {
	Watch(owner client.Object, obj runtime.Object) error
}

type PausingClientObject interface {
	ClientObject() client.Object
	IsObjectPaused(obj client.Object) bool
}

func (r *PhaseReconciler) Reconcile(
	ctx context.Context,
	owner PausingClientObject,
	phase packagesv1alpha1.ObjectPhase,
	probe internalprobe.Interface,
) (failedProbes []string, err error) {

	// Reconcile objects in phase
	for _, phaseObject := range phase.Objects {
		obj, err := unstructuredFromObjectObject(&phaseObject)
		if err != nil {
			return nil, err
		}
		if err := r.reconcileObject(ctx, owner, obj); err != nil {
			return nil, err
		}

		if success, message := probe.Probe(obj); !success {
			gvk := obj.GroupVersionKind()
			failedProbes = append(failedProbes,
				fmt.Sprintf("%s %s %s/%s: %s", gvk.Group, gvk.Kind, obj.GetNamespace(), obj.GetName(), message))
		}
	}
	return failedProbes, nil
}

func (r *PhaseReconciler) reconcileObject(
	ctx context.Context,
	owner PausingClientObject,
	obj *unstructured.Unstructured,
) error {
	log := controllers.LoggerFromContext(ctx)

	// Add our own label
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[ObjectSetLabelKey] = strings.Trim(
		strings.Replace(client.ObjectKeyFromObject(owner.ClientObject()).String(), "/", "-", -1), "-")
	obj.SetLabels(labels)

	// Force namespace to this one if it's set
	if len(obj.GetNamespace()) == 0 {
		obj.SetNamespace(owner.ClientObject().GetNamespace())
	}

	// Ensure we are owner
	if err := r.ownerStrategy.SetControllerReference(owner.ClientObject(), obj, r.scheme); err != nil {
		// if err := controllerutil.SetControllerReference(owner.ClientObject(), obj, r.scheme); err != nil {
		return err
	}

	// Ensure to watchlist
	if err := r.dw.Watch(owner.ClientObject(), obj); err != nil {
		return fmt.Errorf("watching new resource: %w", err)
	}

	currentObj := obj.DeepCopy()
	err := r.client.Get(ctx, client.ObjectKeyFromObject(obj), currentObj)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("getting %s: %w", obj.GroupVersionKind(), err)
	}

	if owner.IsObjectPaused(obj) {
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
	isOwner := r.ownerStrategy.IsOwner(owner.ClientObject(), currentObj)

	// // Let's take over ownership from the other ObjectSet.
	// var newOwnerRefs []metav1.OwnerReference
	// for _, ownerRef := range currentObjOwners {
	// 	ownerRef.Controller = nil
	// 	newOwnerRefs = append(newOwnerRefs, ownerRef)
	// }
	// r.ownerStrategy.SetOwnerReferences(obj, newOwnerRefs)

	if !isOwner {
		// Release other controllers

		// Just patch the OwnerReferences of the object,
		// or we by pass the DeepDerivative check and
		// might update other object properties.
		updatedOwnersObj := currentObj.DeepCopy()
		r.ownerStrategy.ReleaseController(currentObj)

		if err := r.ownerStrategy.SetControllerReference(owner.ClientObject(), updatedOwnersObj, r.scheme); err != nil {
			return err
		}

		log.Info("patching for ownership", "obj", client.ObjectKeyFromObject(obj))
		if err := r.client.Patch(
			ctx, currentObj, client.MergeFrom(updatedOwnersObj)); err != nil {
			return fmt.Errorf("patching Owners: %w", err)
		}
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

func unstructuredFromObjectObject(packageObject *packagesv1alpha1.ObjectSetObject) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(packageObject.Object.Raw, obj); err != nil {
		return nil, fmt.Errorf("converting RawExtension into unstructured: %w", err)
	}
	return obj, nil
}
