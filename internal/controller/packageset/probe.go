package packageset

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	internalprobe "github.com/thetechnick/package-operator/internal/probe"
)

type probeFailure struct {
	ProbeName string
	Object    client.Object
}

func (pf probeFailure) String() string {
	return fmt.Sprintf(
		"Probe %q failed on %s %s/%s",
		pf.ProbeName,
		pf.Object.GetObjectKind().GroupVersionKind(),
		pf.Object.GetNamespace(),
		pf.Object.GetName(),
	)
}

func parseProbes(
	probesSettings []packagesv1alpha1.PackageProbe,
) []internalprobe.NamedProbe {
	var probes []internalprobe.NamedProbe
	for _, probeSetting := range probesSettings {
		// main probe type
		var probe internalprobe.Interface
		switch probeSetting.Probe.Type {
		case packagesv1alpha1.ProbeCondition:
			if probeSetting.Probe.Condition == nil {
				continue
			}

			probe = &internalprobe.ConditionProbe{
				Type:   probeSetting.Probe.Condition.Type,
				Status: probeSetting.Probe.Condition.Status,
			}

		case packagesv1alpha1.ProbeFieldsEqual:
			if probeSetting.Probe.FieldsEqual == nil {
				continue
			}

			probe = &internalprobe.FieldsEqualProbe{
				FieldA: probeSetting.Probe.FieldsEqual.FieldA,
				FieldB: probeSetting.Probe.FieldsEqual.FieldB,
			}
		}

		// wrap filter type
		switch probeSetting.Selector.Type {
		case packagesv1alpha1.ProbeSelectorKind:
			if probeSetting.Selector.Kind == nil {
				continue
			}

			probe = &internalprobe.KindSelector{
				Interface: probe,
				GroupKind: schema.GroupKind{
					Group: probeSetting.Selector.Kind.Group,
					Kind:  probeSetting.Selector.Kind.Kind,
				},
			}
		}

		probes = append(probes, internalprobe.NamedProbe{
			Name:      probeSetting.Name,
			Interface: probe,
		})
	}
	return probes
}
