package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ClusterPackageSpec defines the desired state of a ClusterPackage.
type ClusterPackageSpec struct {
	PackageSourceSpec `json:",inline"`
}

// ClusterPackageStatus defines the observed state of a ClusterPackage
type ClusterPackageStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// DEPRECATED: This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// Human readable status - please use .Conditions from code
	Phase PackageStatusPhase `json:"phase,omitempty"`
	// Hash of the PackageSourceSpec, used to track whether a new unpack is needed.
	SourceHash string `json:"sourceHash,omitempty"`
}

// ClusterPackage is the Schema for the ClusterPackages API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterPackage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterPackageSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status ClusterPackageStatus `json:"status,omitempty"`
}

// ClusterPackageList contains a list of ClusterPackages
// +kubebuilder:object:root=true
type ClusterPackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterPackage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterPackage{}, &ClusterPackageList{})
}
