package packages

import (
	"fmt"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	internalprobe "github.com/thetechnick/package-operator/internal/probe"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	CacheFinalizer    = "packages.thetechnick.ninja/object-set-cache"
	ObjectSetLabelKey = "packages.thetechnick.ninja/object-set"
)

func ParseProbes(
	packageProbes []packagesv1alpha1.ObjectSetProbe,
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

func PausedObjectMatches(ppo packagesv1alpha1.ObjectSetPausedObject, obj client.Object) bool {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Group == ppo.Group &&
		gvk.Kind == ppo.Kind &&
		obj.GetName() == ppo.Name {
		return true
	}
	return false
}

func UnstructuredFromObjectObject(packageObject *packagesv1alpha1.ObjectSetObject) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(packageObject.Object.Raw, obj); err != nil {
		return nil, fmt.Errorf("converting RawExtension into unstructured: %w", err)
	}
	return obj, nil
}
