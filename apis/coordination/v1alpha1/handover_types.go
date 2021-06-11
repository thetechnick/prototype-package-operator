package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
)

// HandoverSpec defines the desired state of a Handover.
type HandoverSpec struct {
	Strategy HandoverStrategy         `json:"strategy"`
	Target   HandoverTarget           `json:"target"`
	Probes   []packagesv1alpha1.Probe `json:"probes"`
}

type HandoverStrategy struct {
	Type    HandoverStrategyType         `json:"type"`
	Relabel *HandoverStrategyRelabelSpec `json:"relabel,omitempty"`
}

type HandoverStrategyType string

const (
	HandoverStrategyRelabel HandoverStrategyType = "Relabel"
)

type HandoverStrategyRelabelSpec struct {
	Label    string `json:"label"`
	OldValue string `json:"oldValue"`
	NewValue string `json:"newValue"`
}

type HandoverTarget struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
}

// HandoverStatus defines the observed state of a Handover
type HandoverStatus struct {
	Processing []HandoverRef       `json:"processing,omitempty"`
	Stats      HandoverStatusStats `json:"stats,omitempty"`
	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// DEPRECATED: This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// Human readable status - please use .Conditions from code
	Phase HandoverPhase `json:"phase,omitempty"`
}

type HandoverStatusStats struct {
	// +optional
	Found int32 `json:"found"`
	// +optional
	Available int32 `json:"available"`
	// +optional
	Updated int32 `json:"updated"`
}

type HandoverRef struct {
	UID  types.UID `json:"uid"`
	Name string    `json:"name"`
}

const (
	HandoverCompleted = "Completed"
)

type HandoverPhase string

// Well-known Handover Phases for printing a Status in kubectl,
// see deprecation notice in HandoverStatus for details.
const (
	HandoverPhasePending     HandoverPhase = "Pending"
	HandoverPhaseProgressing HandoverPhase = "Progressing"
	HandoverPhaseCompleted   HandoverPhase = "Completed"
)

// Handover controls the handover process between two operators.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Found",type="integer",JSONPath=".status.stats.found"
// +kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.stats.available"
// +kubebuilder:printcolumn:name="Updated",type="integer",JSONPath=".status.stats.updated"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Handover struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HandoverSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status HandoverStatus `json:"status,omitempty"`
}

// HandoverList contains a list of Handovers
// +kubebuilder:object:root=true
type HandoverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Handover `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Handover{}, &HandoverList{})
}
