package controller

import (
	"encoding/binary"
	"fmt"
	"hash"
	"hash/fnv"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/yaml"

	"github.com/davecgh/go-spew/spew"
	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
)

func unstructuredFromPackageObject(packageObject *packagesv1alpha1.PackageObject) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(packageObject.Object.Raw, obj); err != nil {
		return nil, fmt.Errorf("converting RawExtension into unstructured: %w", err)
	}
	return obj, nil
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

// returns the latest revision among the given PackageSets.
// expects the input packageSet list to be already sorted by revision.
func latestRevision(packageSets []packagesv1alpha1.PackageSet) (int, error) {
	if len(packageSets) == 0 {
		return 0, nil
	}
	latestPackageSet := packageSets[len(packageSets)-1]
	if len(latestPackageSet.Annotations[packageSetRevisionAnnotation]) == 0 {
		return 0, nil
	}
	return strconv.Atoi(latestPackageSet.Annotations[packageSetRevisionAnnotation])
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
				Group: obj.GroupVersionKind().Group,
				Kind:  obj.GetKind(),
				Name:  obj.GetName(),
			})
		}
	}
	return pausedObject, nil
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

func isOwnerOf(owner, obj client.Object, scheme *runtime.Scheme) (bool, error) {
	ownerGVK, err := apiutil.GVKForObject(owner, scheme)
	if err != nil {
		return false, err
	}
	for _, ownerRef := range obj.GetOwnerReferences() {
		if ownerRef.Kind == ownerGVK.Kind &&
			ownerRef.APIVersion == ownerGVK.Group &&
			ownerRef.Controller != nil &&
			*ownerRef.Controller {
			return true, nil
		}
	}
	return false, nil
}
