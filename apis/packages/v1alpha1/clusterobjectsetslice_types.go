package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ClusterObjectSetSlice holds a collection of objects too large to inline into the parent ObjectSet.
// Multiple ClusterObjectSetSlices may provide the storage backend for particularly large ObjectSets.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterObjectSetSlice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Objects belonging to this phase.
	Objects []ObjectSetObject `json:"objects"`
}

// ClusterObjectSetSliceList contains a list of ClusterObjectSetSlices
// +kubebuilder:object:root=true
type ClusterObjectSetSliceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterObjectSetSlice `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterObjectSetSlice{}, &ClusterObjectSetSliceList{})
}
