package packagedeployment

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash"
	"hash/fnv"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

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

const packageSetHashAnnotation = "packages.thetechnick.ninja/hash"

func (r *PackageDeploymentReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("PackageDeployment", req.NamespacedName.String())

	packageDeployment := &packagesv1alpha1.PackageDeployment{}
	if err := r.Get(ctx, req.NamespacedName, packageDeployment); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	templateHash := computeHash(
		&packageDeployment.Spec.Template,
		packageDeployment.Status.CollisionCount)

	desiredPackageSet := &packagesv1alpha1.PackageSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        packageDeployment.Name + "-" + templateHash,
			Namespace:   packageDeployment.Namespace,
			Annotations: packageDeployment.Annotations,
			Labels:      packageDeployment.Spec.Template.Metadata.Labels,
		},
		Spec: packagesv1alpha1.PackageSetSpec{
			Phases: packageDeployment.
				Spec.Template.Spec.Phases,
			ReadinessProbes: packageDeployment.
				Spec.Template.Spec.ReadinessProbes,
		},
	}
	if desiredPackageSet.Annotations == nil {
		desiredPackageSet.Annotations = map[string]string{}
	}
	desiredPackageSet.Annotations[packageSetHashAnnotation] = templateHash

	if err := controllerutil.SetControllerReference(
		packageDeployment, desiredPackageSet, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	packageSetSelector, err := metav1.LabelSelectorAsSelector(
		&packageDeployment.Spec.Selector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("invalid selector: %w", err)
	}

	packageSetList := &packagesv1alpha1.PackageSetList{}
	if err := r.List(
		ctx, packageSetList,
		client.MatchingLabelsSelector{
			Selector: packageSetSelector,
		},
		client.InNamespace(packageDeployment.Namespace),
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing PackageSets: %w", err)
	}

	pausedObjects, err := pausedObjectsFromPackageSet(desiredPackageSet)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting paused objects from PackageSet: %w", err)
	}

	currentPackageSet, oldPackageSets := sortPackageSets(
		templateHash, packageSetList.Items)
	for _, oldPackageSet := range oldPackageSets {
		if !equality.Semantic.DeepEqual(pausedObjects, oldPackageSet.Spec.PausedFor) {
			oldPackageSet.Spec.PausedFor = pausedObjects
			if err := r.Update(ctx, &oldPackageSet); err != nil {
				return ctrl.Result{}, fmt.Errorf("updating old PackageSet: %w", err)
			}
		}

		if oldPackageSet.Generation != oldPackageSet.Status.ObservedGeneration &&
			!equality.Semantic.DeepDerivative(pausedObjects, oldPackageSet.Status.PausedFor) {
			log.Info(
				"waiting for PackageSet to be paused",
				"PackageSet", client.ObjectKeyFromObject(&oldPackageSet).String())
			return ctrl.Result{}, nil
		}
	}

	if currentPackageSet == nil {
		packageDeployment.Status.Phase = packagesv1alpha1.PackageDeploymentProgressing
		packageDeployment.Status.ObservedGeneration = packageDeployment.Generation
		if err := r.Status().Update(ctx, packageDeployment); err != nil {
			return ctrl.Result{}, err
		}

		err := r.Create(ctx, desiredPackageSet)
		if errors.IsAlreadyExists(err) {
			// TODO: hash collision detection
			return ctrl.Result{}, nil
		}
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("creating new PackageSet: %w", err)
		}
		return ctrl.Result{}, nil
	}

	if meta.IsStatusConditionTrue(
		currentPackageSet.Status.Conditions,
		packagesv1alpha1.PackageSetAvailable,
	) {
		// for _, oldPackageSet := range oldPackageSets {
		// 	if err := r.Delete(ctx, &oldPackageSet); err != nil &&
		// 		!errors.IsNotFound(err) {
		// 		return ctrl.Result{}, fmt.Errorf("deleting old PackageSet: %w", err)
		// 	}
		// }
	} else {
		meta.SetStatusCondition(&packageDeployment.Status.Conditions, metav1.Condition{
			Type:               packagesv1alpha1.PackageSetAvailable,
			Status:             metav1.ConditionFalse,
			Reason:             "PackageSetUnready",
			Message:            "xxx",
			ObservedGeneration: packageDeployment.Generation,
		})
		packageDeployment.Status.Phase = packagesv1alpha1.PackageDeploymentPhaseNotReady
		packageDeployment.Status.ObservedGeneration = packageDeployment.Generation
		return ctrl.Result{}, r.Status().Update(ctx, packageDeployment)
	}

	meta.SetStatusCondition(&packageDeployment.Status.Conditions, metav1.Condition{
		Type:               packagesv1alpha1.PackageDeploymentAvailable,
		Status:             metav1.ConditionTrue,
		Reason:             "Available",
		Message:            "Latest PackageSet is available.",
		ObservedGeneration: packageDeployment.Generation,
	})
	packageDeployment.Status.Phase = packagesv1alpha1.PackageDeploymentAvailable
	packageDeployment.Status.ObservedGeneration = packageDeployment.Generation
	if err := r.Status().Update(ctx, packageDeployment); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func pausedObjectsFromPackageSet(packageSet *packagesv1alpha1.PackageSet) ([]packagesv1alpha1.PackagePausedObject, error) {
	var pausedObject []packagesv1alpha1.PackagePausedObject
	for _, phase := range packageSet.Spec.Phases {
		for _, phaseObject := range phase.Objects {
			obj := &unstructured.Unstructured{}
			if err := yaml.Unmarshal(phaseObject.Object.Raw, obj); err != nil {
				return nil, fmt.Errorf("converting RawExtension into unstructured: %w", err)
			}
			pausedObject = append(pausedObject, packagesv1alpha1.PackagePausedObject{
				Group:     obj.GroupVersionKind().Group,
				Kind:      obj.GetKind(),
				Name:      obj.GetName(),
				Namespace: packageSet.GetNamespace(),
			})
		}
	}
	return pausedObject, nil
}

func sortPackageSets(
	templateHash string,
	packageSets []packagesv1alpha1.PackageSet,
) (
	current *packagesv1alpha1.PackageSet,
	rest []packagesv1alpha1.PackageSet,
) {
	for _, packageSet := range packageSets {
		if packageSet.Annotations[packageSetHashAnnotation] == templateHash {
			current = packageSet.DeepCopy()
			continue
		}
		rest = append(rest, packageSet)
	}
	return
}

// computeHash returns a hash value calculated from pod template and
// a collisionCount to avoid hash collision. The hash will be safe encoded to
// avoid bad words.
func computeHash(template *packagesv1alpha1.PackageSetTemplate, collisionCount *int32) string {
	hasher := fnv.New32a()
	deepHashObject(hasher, *template)

	// Add collisionCount in the hash if it exists.
	if collisionCount != nil {
		collisionCountBytes := make([]byte, 8)
		binary.LittleEndian.PutUint32(
			collisionCountBytes, uint32(*collisionCount))
		hasher.Write(collisionCountBytes)
	}

	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}

// deepHashObject writes specified object to hash using the spew library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
func deepHashObject(hasher hash.Hash, objectToWrite interface{}) {
	hasher.Reset()
	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}
	printer.Fprintf(hasher, "%#v", objectToWrite)
}
