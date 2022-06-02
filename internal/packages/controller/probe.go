package controller

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	internalprobe "github.com/thetechnick/package-operator/internal/probe"
)

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
