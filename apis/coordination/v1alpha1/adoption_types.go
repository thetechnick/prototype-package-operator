package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AdoptionSpec defines the desired state of a Adoption.
type AdoptionSpec struct {
	// Strategy to use for adoption.
	Strategy AdoptionStrategy `json:"strategy"`
	// TargetAPI to use for adoption.
	TargetAPI TargetAPI `json:"targetAPI"`
}

// AdoptionStrategy defines the strategy to handover objects.
type AdoptionStrategy struct {
	// Type of adoption strategy. Can be "Static", "RoundRobin".
	// +kubebuilder:default=Static
	// +kubebuilder:validation:Enum={"Static","RoundRobin"}
	Type AdoptionStrategyType `json:"type"`

	// Static adoption strategy configuration.
	// Only present when type=Static.
	Static *AdoptionStrategyStaticSpec `json:"static,omitempty"`

	// RoundRobin adoption strategy configuration.
	// Only present when type=RoundRobin.
	RoundRobin *AdoptionStrategyRoundRobinSpec `json:"roundRobin,omitempty"`
}

type AdoptionStrategyType string

const (
	// Static will change a specified label object after object.
	AdoptionStrategyStatic AdoptionStrategyType = "Static"
	// RoundRobin will apply given labels via a round robin strategy.
	AdoptionStrategyRoundRobin AdoptionStrategyType = "RoundRobin"
)

type AdoptionStrategyStaticSpec struct {
	// Labels to set on objects.
	Labels map[string]string `json:"labels"`
}

type AdoptionStrategyRoundRobinSpec struct {
	// Labels to set always, no matter the round robin choice.
	Always map[string]string `json:"always"`
	// Options for the round robin strategy to choose from.
	// Only a single label set of all the provided options will be applied.
	Options []map[string]string `json:"options"`
}

// AdoptionStatus defines the observed state of a Adoption
type AdoptionStatus struct {
	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// DEPRECATED: This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// Human readable status - please use .Conditions from code
	Phase AdoptionPhase `json:"phase,omitempty"`
	// Tracks round robin state to restart where the last operation ended.
	RoundRobin *AdoptionRoundRobinStatus `json:"roundRobin,omitempty"`
}

type AdoptionRoundRobinStatus struct {
	// Last index chosen by the round robin algorithm.
	LastIndex int `json:"lastIndex"`
}

const (
	// The active condition is True as long as the adoption process is active.
	AdoptionActive = "Active"
)

type AdoptionPhase string

// Well-known Adoption Phases for printing a Status in kubectl,
// see deprecation notice in AdoptionStatus for details.
const (
	AdoptionPhasePending AdoptionPhase = "Pending"
	AdoptionPhaseActive  AdoptionPhase = "Active"
)

// Adoption controls the assignment of new objects to an operator.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Adoption struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AdoptionSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status AdoptionStatus `json:"status,omitempty"`
}

// AdoptionList contains a list of Adoptions
// +kubebuilder:object:root=true
type AdoptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Adoption `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Adoption{}, &AdoptionList{})
}
