package probe

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type KindSelector struct {
	Interface
	schema.GroupKind
}

func (kp *KindSelector) Probe(obj *unstructured.Unstructured) bool {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if kp.Kind == gvk.Kind &&
		kp.Group == gvk.Group {
		return kp.Interface.Probe(obj)
	}

	// don't probe stuff that does not match
	return true
}
