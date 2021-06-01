package packageset

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	internalprobe "github.com/thetechnick/package-operator/internal/controller/packageset/probe"
)

func parseProbes(
	packageProbes []packagesv1alpha1.PackageProbe,
) []internalprobe.Interface {
	var probes []internalprobe.Interface
	for _, packageProbe := range packageProbes {
		for _, probeSpec := range packageProbe.Probes {
			// main probe type
			var probe internalprobe.Interface
			switch probeSpec.Type {
			case packagesv1alpha1.ProbeCondition:
				if probeSpec.Condition == nil {
					continue
				}

				probe = &internalprobe.ConditionProbe{
					Type:   probeSpec.Condition.Type,
					Status: probeSpec.Condition.Status,
				}

			case packagesv1alpha1.ProbeFieldsEqual:
				if probeSpec.FieldsEqual == nil {
					continue
				}

				probe = &internalprobe.FieldsEqualProbe{
					FieldA: probeSpec.FieldsEqual.FieldA,
					FieldB: probeSpec.FieldsEqual.FieldB,
				}

			default:
				// Unknown probe type
				continue
			}

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
	}
	return probes
}
