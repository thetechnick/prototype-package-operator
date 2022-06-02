package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
)

// ClusterHandoverSpec defines the desired state of a ClusterHandover.
type ClusterHandoverSpec struct {
	// Strategy to use when handing over objects between operators.
	Strategy HandoverStrategy `json:"strategy"`
	// TargetAPI to use for handover.
	TargetAPI TargetAPI `json:"targetAPI"`
	// Probes to check selected objects for availability.
	Probes []packagesv1alpha1.Probe `json:"probes"`
}

// ClusterHandoverStatus defines the observed state of a ClusterHandover
type ClusterHandoverStatus struct {
	// Processing set of objects during handover.
	Processing []HandoverRef `json:"processing,omitempty"`
	// Statistics of the handover process.
	Stats HandoverStatusStats `json:"stats,omitempty"`
	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// DEPRECATED: This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// Human readable status - please use .Conditions from code
	Phase ClusterHandoverPhase `json:"phase,omitempty"`
}

const (
	ClusterHandoverCompleted = "Completed"
)

type ClusterHandoverPhase string

// Well-known ClusterHandover Phases for printing a Status in kubectl,
// see deprecation notice in ClusterHandoverStatus for details.
const (
	ClusterHandoverPhasePending     ClusterHandoverPhase = "Pending"
	ClusterHandoverPhaseProgressing ClusterHandoverPhase = "Progressing"
	ClusterHandoverPhaseCompleted   ClusterHandoverPhase = "Completed"
)

// ClusterHandover controls the handover process between two operators.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Found",type="integer",JSONPath=".status.stats.found"
// +kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.stats.available"
// +kubebuilder:printcolumn:name="Updated",type="integer",JSONPath=".status.stats.updated"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterHandover struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterHandoverSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status ClusterHandoverStatus `json:"status,omitempty"`
}

// ClusterHandoverList contains a list of ClusterHandovers
// +kubebuilder:object:root=true
type ClusterHandoverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterHandover `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterHandover{}, &ClusterHandoverList{})
}
