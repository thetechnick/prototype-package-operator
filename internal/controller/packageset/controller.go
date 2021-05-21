package packageset

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/yaml"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	internalprobe "github.com/thetechnick/package-operator/internal/probe"
)

type PackageSetReconciler struct {
	client.Client
	DynamicClient   dynamic.Interface
	DiscoveryClient *discovery.DiscoveryClient
	Log             logr.Logger
	Scheme          *runtime.Scheme

	dw *dynamicWatcher
}

func (r *PackageSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.dw = newDynamicWatcher(r.Log, r.Scheme, r.RESTMapper(), r.DynamicClient)

	return ctrl.NewControllerManagedBy(mgr).
		For(&packagesv1alpha1.PackageSet{}).
		Watches(r.dw, &handler.EnqueueRequestForOwner{
			OwnerType:    &packagesv1alpha1.PackageSet{},
			IsController: false,
		}).
		Complete(r)
}

const (
	packageSetCacheFinalizer = "packages.thetechnick.ninja/package-set-cache"
	packageSetLabel          = "packages.thetechnick.ninja/package-set"
)

func (r *PackageSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("PackageSet", req.NamespacedName.String())

	packageSet := &packagesv1alpha1.PackageSet{}
	if err := r.Get(ctx, req.NamespacedName, packageSet); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !packageSet.DeletionTimestamp.IsZero() {
		// PackgeSet was deleted.
		return ctrl.Result{}, r.handleDeletion(ctx, packageSet)
	}

	// Add finalizers
	if !controllerutil.ContainsFinalizer(
		packageSet, packageSetCacheFinalizer) {
		controllerutil.AddFinalizer(packageSet, packageSetCacheFinalizer)
		if err := r.Update(ctx, packageSet); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer: %w", err)
		}
	}

	if packageSet.Spec.Archived {
		// Archive handling
		return ctrl.Result{}, r.handleArchived(ctx, packageSet)
	}

	// Dependencies
	stop, err := r.checkDependencies(ctx, packageSet)
	if err != nil {
		return ctrl.Result{}, err
	}
	if stop {
		// TODO: find a better Requeue Time
		log.Info("dependency check failed", "nextCheck", 30*time.Second)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Probes
	probes := parseProbes(packageSet.Spec.ReadinessProbes)
	for _, phase := range packageSet.Spec.Phases {
		stop, err := r.reconcilePhase(ctx, packageSet, &phase, probes, log)
		if err != nil {
			return ctrl.Result{}, err
		}
		if stop {
			return ctrl.Result{}, nil
		}
	}

	if packageSet.Spec.Paused {
		meta.SetStatusCondition(&packageSet.Status.Conditions, metav1.Condition{
			Type:               packagesv1alpha1.PackageSetPaused,
			Status:             metav1.ConditionTrue,
			Reason:             "Paused",
			Message:            "PackageSet is paused.",
			ObservedGeneration: packageSet.Generation,
		})
	} else {
		meta.RemoveStatusCondition(
			&packageSet.Status.Conditions, packagesv1alpha1.PackageSetPaused)
	}

	meta.RemoveStatusCondition(&packageSet.Status.Conditions, packagesv1alpha1.PackageSetArchived)
	meta.SetStatusCondition(&packageSet.Status.Conditions, metav1.Condition{
		Type:               packagesv1alpha1.PackageSetAvailable,
		Status:             metav1.ConditionTrue,
		Reason:             "Available",
		Message:            "Package is available and passes all probes.",
		ObservedGeneration: packageSet.Generation,
	})
	packageSet.Status.PausedFor = packageSet.Spec.PausedFor
	packageSet.Status.Phase = packagesv1alpha1.PackageSetAvailable
	packageSet.Status.ObservedGeneration = packageSet.Generation
	if err := r.Status().Update(ctx, packageSet); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *PackageSetReconciler) checkDependencies(
	ctx context.Context,
	packageSet *packagesv1alpha1.PackageSet,
) (stop bool, err error) {
	if len(packageSet.Spec.Dependencies) == 0 {
		return false, nil
	}

	_, apiResourceLists, err := r.DiscoveryClient.ServerGroupsAndResources()
	if err != nil {
		return false, fmt.Errorf("discovering available APIs: %w", err)
	}

	var missingGKV []string
	for _, dependency := range packageSet.Spec.Dependencies {
		if dependency.KubernetesAPI == nil {
			continue
		}

		gvk := schema.GroupVersionKind{
			Group:   dependency.KubernetesAPI.Group,
			Version: dependency.KubernetesAPI.Version,
			Kind:    dependency.KubernetesAPI.Kind,
		}
		if !hasGVK(gvk, apiResourceLists) {
			missingGKV = append(missingGKV, gvk.String())
		}
	}

	if len(missingGKV) > 0 {
		meta.SetStatusCondition(&packageSet.Status.Conditions, metav1.Condition{
			Type:               packagesv1alpha1.PackageSetAvailable,
			Status:             metav1.ConditionFalse,
			Reason:             "MissingDependency",
			Message:            fmt.Sprintf("Missing objects in kubernetes API: %s", strings.Join(missingGKV, ", ")),
			ObservedGeneration: packageSet.Generation,
		})
		packageSet.Status.Phase = packagesv1alpha1.PackageSetPhaseMissingDependency
		packageSet.Status.ObservedGeneration = packageSet.Generation
		if err := r.Status().Update(ctx, packageSet); err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}

func hasGVK(gvk schema.GroupVersionKind, apiResourceLists []*metav1.APIResourceList) bool {
	for _, apiResourceList := range apiResourceLists {
		if apiResourceList == nil {
			continue
		}

		if gvk.GroupVersion().String() == apiResourceList.GroupVersion {
			for _, apiResource := range apiResourceList.APIResources {
				if gvk.Kind == apiResource.Kind {
					return true
				}
			}
		}
	}
	return false
}

func (r *PackageSetReconciler) reconcilePhase(
	ctx context.Context,
	packageSet *packagesv1alpha1.PackageSet,
	phase *packagesv1alpha1.PackagePhase,
	probes []internalprobe.NamedProbe,
	log logr.Logger,
) (stop bool, err error) {
	var failedProbes []string

	// Reconcile objects in phase
	for _, phaseObject := range phase.Objects {
		obj, err := unstructuredFromPackageObject(&phaseObject)
		if err != nil {
			return false, err
		}
		if err := r.reconcileObject(ctx, packageSet, obj, log); err != nil {
			return false, err
		}

		for _, probe := range probes {
			if !probe.Probe(obj) {
				failedProbes = append(failedProbes, probeFailure{
					ProbeName: probe.GetName(),
					Object:    obj,
				}.String())
			}
		}
	}

	if len(failedProbes) == 0 {
		return false, nil
	}

	meta.SetStatusCondition(&packageSet.Status.Conditions, metav1.Condition{
		Type:               packagesv1alpha1.PackageSetAvailable,
		Status:             metav1.ConditionFalse,
		Reason:             "ProbeFailure",
		Message:            fmt.Sprintf("Phase %q failed: %s", phase.Name, strings.Join(failedProbes, ", ")),
		ObservedGeneration: packageSet.Generation,
	})
	packageSet.Status.Phase = packagesv1alpha1.PackageSetPhaseNotReady
	packageSet.Status.ObservedGeneration = packageSet.Generation
	if err := r.Status().Update(ctx, packageSet); err != nil {
		return false, err
	}
	return true, nil
}

// handle deletion of the PackageSet
func (r *PackageSetReconciler) handleDeletion(
	ctx context.Context,
	packageSet *packagesv1alpha1.PackageSet,
) error {
	if controllerutil.ContainsFinalizer(
		packageSet, packageSetCacheFinalizer) {
		controllerutil.RemoveFinalizer(
			packageSet, packageSetCacheFinalizer)

		if err := r.Update(ctx, packageSet); err != nil {
			return fmt.Errorf("removing finalizer: %w", err)
		}
	}

	if err := r.dw.Free(packageSet); err != nil {
		return fmt.Errorf("free cache: %w", err)
	}
	return nil
}

func (r *PackageSetReconciler) handleArchived(
	ctx context.Context,
	packageSet *packagesv1alpha1.PackageSet,
) error {
	for _, phase := range packageSet.Spec.Phases {
		for _, phaseObject := range phase.Objects {
			obj, err := unstructuredFromPackageObject(&phaseObject)
			if err != nil {
				return err
			}
			obj.SetNamespace(packageSet.Namespace)

			if !isPaused(packageSet, obj) {
				if err := r.Delete(ctx, obj); err != nil && !errors.IsNotFound(err) {
					return err
				}
			}
		}
	}

	meta.RemoveStatusCondition(&packageSet.Status.Conditions, packagesv1alpha1.PackageSetPaused)
	meta.RemoveStatusCondition(&packageSet.Status.Conditions, packagesv1alpha1.PackageSetAvailable)
	meta.SetStatusCondition(&packageSet.Status.Conditions, metav1.Condition{
		Type:               packagesv1alpha1.PackageSetArchived,
		Status:             metav1.ConditionTrue,
		Reason:             "Archived",
		Message:            "PackageSet is archived.",
		ObservedGeneration: packageSet.Generation,
	})
	packageSet.Status.PausedFor = packageSet.Spec.PausedFor
	packageSet.Status.Phase = packagesv1alpha1.PackageSetArchived
	packageSet.Status.ObservedGeneration = packageSet.Generation
	if err := r.Status().Update(ctx, packageSet); err != nil {
		return err
	}
	if err := r.dw.Free(packageSet); err != nil {
		return fmt.Errorf("free cache: %w", err)
	}
	return nil
}

func (r *PackageSetReconciler) reconcileObject(
	ctx context.Context,
	packageSet *packagesv1alpha1.PackageSet,
	obj *unstructured.Unstructured,
	log logr.Logger,
) error {
	// Add our own label
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[packageSetLabel] = strings.Replace(client.ObjectKeyFromObject(packageSet).String(), "/", "-", -1)
	obj.SetLabels(labels)

	// Force namespace to this one
	obj.SetNamespace(packageSet.Namespace)

	// Ensure we are owner
	if err := controllerutil.SetControllerReference(packageSet, obj, r.Scheme); err != nil {
		return err
	}

	// Ensure to watchlist
	if err := r.dw.Watch(packageSet, obj); err != nil {
		return fmt.Errorf("watching new resource: %w", err)
	}

	currentObj := obj.DeepCopy()
	err := r.Get(ctx, client.ObjectKeyFromObject(obj), currentObj)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("getting: %w", err)
	}

	if isPaused(packageSet, obj) {
		// Paused, don't reconcile.
		// Just report the latest object state.
		*obj = *currentObj
		return nil
	}

	if errors.IsNotFound(err) {
		err := r.Create(ctx, obj)
		if err != nil {
			return fmt.Errorf("creating: %w", err)
		}
	}

	// Adoption/Handover process
	var isOwner bool
	for _, ownerRef := range currentObj.GetOwnerReferences() {
		isOwner = ownerRef.UID == packageSet.UID
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
		if err := r.Patch(ctx, currentObj, client.MergeFrom(updatedOwnersObj)); err != nil {
			return fmt.Errorf("patching Owners: %w", err)
		}
	}

	if err := controllerutil.SetControllerReference(packageSet, obj, r.Scheme); err != nil {
		return err
	}

	// Update
	if !equality.Semantic.DeepDerivative(obj.Object, currentObj.Object) {
		log.Info("patching spec", "obj", client.ObjectKeyFromObject(obj))
		// this is only updating "known" fields,
		// so annotations/labels and other properties will be preserved.
		err := r.Patch(
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

func unstructuredFromPackageObject(packageObject *packagesv1alpha1.PackageObject) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(packageObject.Object.Raw, obj); err != nil {
		return nil, fmt.Errorf("converting RawExtension into unstructured: %w", err)
	}
	return obj, nil
}

func isPaused(packageSet *packagesv1alpha1.PackageSet, obj client.Object) bool {
	if packageSet.Spec.Paused {
		return true
	}

	for _, pausedObject := range packageSet.Spec.PausedFor {
		if pausedObjectMatches(pausedObject, obj) {
			return true
		}
	}
	return false
}

func pausedObjectMatches(ppo packagesv1alpha1.PackagePausedObject, obj client.Object) bool {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Group == ppo.Group &&
		gvk.Kind == ppo.Kind &&
		obj.GetName() == ppo.Name {
		return true
	}
	return false
}
