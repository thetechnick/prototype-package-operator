package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterAdoptionSpec defines the desired state of a ClusterAdoption.
type ClusterAdoptionSpec struct {
	// Strategy to use for adoption.
	Strategy ClusterAdoptionStrategy `json:"strategy"`
	// TargetAPI to use for adoption.
	TargetAPI TargetAPI `json:"targetAPI"`
}

// ClusterAdoptionStrategy defines the strategy to handover objects.
type ClusterAdoptionStrategy struct {
	// Type of handover strategy. Can be "Static".
	// +kubebuilder:default=Static
	// +kubebuilder:validation:Enum={"Static","RoundRobin"}
	Type ClusterAdoptionStrategyType `json:"type"`

	// Static handover strategy configuration.
	// Only present when type=Static.
	Static *AdoptionStrategyStaticSpec `json:"static,omitempty"`

	// RoundRobin adoption strategy configuration.
	// Only present when type=RoundRobin.
	RoundRobin *AdoptionStrategyRoundRobinSpec `json:"roundRobin,omitempty"`
}

type ClusterAdoptionStrategyType string

const (
	// Static will change a specified label object after object.
	ClusterAdoptionStrategyStatic ClusterAdoptionStrategyType = "Static"
	// RoundRobin will apply given labels via a round robin strategy.
	ClusterAdoptionStrategyRoundRobin ClusterAdoptionStrategyType = "RoundRobin"
)

// ClusterAdoptionStatus defines the observed state of a ClusterAdoption
type ClusterAdoptionStatus struct {
	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// DEPRECATED: This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// Human readable status - please use .Conditions from code
	Phase ClusterAdoptionPhase `json:"phase,omitempty"`
	// Tracks round robin state to restart where the last operation ended.
	RoundRobin *AdoptionRoundRobinStatus `json:"roundRobin,omitempty"`
}

const (
	// The active condition is True as long as the adoption process is active.
	ClusterAdoptionActive = "Active"
)

type ClusterAdoptionPhase string

// Well-known ClusterAdoption Phases for printing a Status in kubectl,
// see deprecation notice in ClusterAdoptionStatus for details.
const (
	ClusterAdoptionPhasePending ClusterAdoptionPhase = "Pending"
	ClusterAdoptionPhaseActive  ClusterAdoptionPhase = "Active"
)

// ClusterAdoption controls the assignment of new objects to an operator.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterAdoption struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterAdoptionSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status ClusterAdoptionStatus `json:"status,omitempty"`
}

// ClusterAdoptionList contains a list of ClusterAdoptions
// +kubebuilder:object:root=true
type ClusterAdoptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAdoption `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterAdoption{}, &ClusterAdoptionList{})
}
