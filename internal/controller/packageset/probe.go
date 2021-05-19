package packageset

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/equality"
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

		case packagesv1alpha1.ProbeFieldsEqual:
			if probeSetting.Probe.FieldsEqual == nil {
				continue
			}

			probe = &fieldsEqualProbe{
				name:   probeSetting.Name,
				fieldA: probeSetting.Probe.FieldsEqual.FieldA,
				fieldB: probeSetting.Probe.FieldsEqual.FieldB,
			}
		}

		// wrap filter type
		switch probeSetting.Selector.Type {
		case packagesv1alpha1.ProbeSelectorKind:
			if probeSetting.Selector.Kind == nil {
				continue
			}

			probe = &kindProbe{
				probe: probe,
				GroupKind: schema.GroupKind{
					Group: probeSetting.Selector.Kind.Group,
					Kind:  probeSetting.Selector.Kind.Kind,
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

	if observedGeneration, ok, err := unstructured.NestedInt64(obj.Object, "status", "observedGeneration"); err == nil && ok && observedGeneration != obj.GetGeneration() {
		return false
	}

	for _, condI := range conditions {
		cond, ok := condI.(map[string]interface{})
		if !ok {
			continue
		}

		if cond["type"] == cp.Type &&
			cond["status"] == cp.Status {
			if observedGeneration, ok, err := unstructured.NestedInt64(cond, "observedGeneration"); err == nil && ok && observedGeneration != obj.GetGeneration() {
				return false
			}

			return true
		}
	}
	return false
}

type fieldsEqualProbe struct {
	name           string
	fieldA, fieldB string
}

func (fe *fieldsEqualProbe) Name() string {
	return fe.name
}

func (fe *fieldsEqualProbe) Probe(obj *unstructured.Unstructured) bool {
	if observedGeneration, ok, err := unstructured.NestedInt64(obj.Object, "status", "observedGeneration"); err == nil && ok && observedGeneration != obj.GetGeneration() {
		return false
	}

	fieldAPath := strings.Split(strings.Trim(fe.fieldA, "."), ".")
	fieldBPath := strings.Split(strings.Trim(fe.fieldB, "."), ".")

	fieldAVal, ok, err := unstructured.NestedFieldCopy(obj.Object, fieldAPath...)
	if err != nil || !ok {
		return false
	}
	fieldBVal, ok, err := unstructured.NestedFieldCopy(obj.Object, fieldBPath...)
	if err != nil || !ok {
		return false
	}

	return equality.Semantic.DeepEqual(fieldAVal, fieldBVal)
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
