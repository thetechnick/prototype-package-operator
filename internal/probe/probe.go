package probe

import (
	"strings"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Interface interface {
	Probe(obj *unstructured.Unstructured) bool
}

// Checks if the objects condition is set and in a certain status.
type ConditionProbe struct {
	Type, Status string
}

func (cp *ConditionProbe) Probe(obj *unstructured.Unstructured) bool {
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
			if observedGeneration, ok, err := unstructured.NestedInt64(cond, "observedGeneration"); err == nil && ok && observedGeneration != obj.GetGeneration() {
				return false
			}

			return true
		}
	}
	return false
}

// Checks if the values of the fields under the given json paths are equal.
type FieldsEqualProbe struct {
	FieldA, FieldB string
}

func (fe *FieldsEqualProbe) Probe(obj *unstructured.Unstructured) bool {
	if observedGeneration, ok, err := unstructured.NestedInt64(obj.Object, "status", "observedGeneration"); err == nil && ok && observedGeneration != obj.GetGeneration() {
		return false
	}

	fieldAPath := strings.Split(strings.Trim(fe.FieldA, "."), ".")
	fieldBPath := strings.Split(strings.Trim(fe.FieldB, "."), ".")

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

// CurrentGenerationProbe ensures that the objects status
// is up to date with the objects generation.
// Requires the probed object to have a .status.observedGeneration property.
type CurrentGenerationProbe struct {
	Interface
}

func (cg *CurrentGenerationProbe) Probe(obj *unstructured.Unstructured) bool {
	if observedGeneration, ok, err := unstructured.NestedInt64(
		obj.Object,
		"status", "observedGeneration",
	); err == nil && ok && observedGeneration != obj.GetGeneration() {
		return false
	}
	return cg.Interface.Probe(obj)
}
