package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ObjectSetLabelName is used to indicate the name of a ObjectSet.
const ObjectSetLabelName = "packages.thetechnick.ninja/object-set-name"

// ObjectSetSlice holds a collection of objects too large to inline into the parent ObjectSet.
// Multiple ObjectSetSlices may provide the storage backend for particularly large ObjectSets.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ObjectSetSlice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Objects belonging to this phase.
	Objects []ObjectSetObject `json:"objects"`
}

// ObjectSetSliceList contains a list of ObjectSetSlices
// +kubebuilder:object:root=true
type ObjectSetSliceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectSetSlice `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ObjectSetSlice{}, &ObjectSetSliceList{})
}
