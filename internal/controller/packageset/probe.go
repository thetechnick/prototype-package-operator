package packageset

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
)

type probe interface {
	Name() string
	Probe(obj *unstructured.Unstructured) bool
}

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

func parseProbes(probesSettings []packagesv1alpha1.PackageProbe) []probe {
	var probes []probe
	for _, probeSetting := range probesSettings {
		// main probe type
		var probe probe
		switch probeSetting.Probe.Type {
		case packagesv1alpha1.ProbeCondition:
			if probeSetting.Probe.Condition == nil {
				continue
			}

			probe = &conditionProbe{
				name:   probeSetting.Name,
				Type:   probeSetting.Probe.Condition.Type,
				Status: probeSetting.Probe.Condition.Status,
			}
		}

		// wrap filter type
		switch probeSetting.Type {
		case packagesv1alpha1.PackageProbeKind:
			if probeSetting.Kind == nil {
				continue
			}

			probe = &kindProbe{
				probe: probe,
				GroupKind: schema.GroupKind{
					Group: probeSetting.Kind.APIGroup,
					Kind:  probeSetting.Kind.Kind,
				},
			}
		}

		probes = append(probes, probe)
	}
	return probes
}

type conditionProbe struct {
	name         string
	Type, Status string
}

func (cp *conditionProbe) Name() string {
	return cp.name
}

func (cp *conditionProbe) Probe(obj *unstructured.Unstructured) bool {
	conditions, exist, err := unstructured.
		NestedSlice(obj.Object, "status", "conditions")
	if err != nil {
		return false
	}
	if !exist {
		return false
	}

	for _, condI := range conditions {
		cond, ok := condI.(map[string]interface{})
		if !ok {
			continue
		}

		if cond["type"] == cp.Type &&
			cond["status"] == cp.Status {
			return true
		}
	}
	return false
}

type kindProbe struct {
	probe
	schema.GroupKind
}

func (kp *kindProbe) Probe(obj *unstructured.Unstructured) bool {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if kp.Kind == gvk.Kind &&
		kp.Group == gvk.Group {
		return kp.probe.Probe(obj)
	}

	// don't probe stuff that does not match
	return true
}
