package objectdeployments

import (
	"fmt"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func pausedObjectsFromPhases(phases []packagesv1alpha1.ObjectPhase) (
	[]packagesv1alpha1.ObjectSetPausedObject, error) {
	var pausedObject []packagesv1alpha1.ObjectSetPausedObject
	for _, phase := range phases {
		for _, phaseObject := range phase.Objects {
			obj := &unstructured.Unstructured{}
			if err := yaml.Unmarshal(phaseObject.Object.Raw, obj); err != nil {
				return nil, fmt.Errorf("converting RawExtension into unstructured: %w", err)
			}
			pausedObject = append(pausedObject, packagesv1alpha1.ObjectSetPausedObject{
				Group: obj.GroupVersionKind().Group,
				Kind:  obj.GetKind(),
				Name:  obj.GetName(),
			})
		}
	}
	return pausedObject, nil
}
