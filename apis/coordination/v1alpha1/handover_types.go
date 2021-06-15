package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
)

// HandoverSpec defines the desired state of a Handover.
type HandoverSpec struct {
	// Strategy to use when handing over objects between operators.
	Strategy HandoverStrategy `json:"strategy"`
	// TargetAPI to use for handover.
	TargetAPI TargetAPI `json:"targetAPI"`
	// Probes to check selected objects for availability.
	Probes []packagesv1alpha1.Probe `json:"probes"`
}

// HandoverStrategy defines the strategy to handover objects.
type HandoverStrategy struct {
	// Type of handover strategy. Can be "Relabel".
	// +kubebuilder:default=Relabel
	// +kubebuilder:validation:Enum={"Relabel"}
	Type HandoverStrategyType `json:"type"`

	// Relabel handover strategy configuration.
	// Only present when type=Relabel.
	Relabel *HandoverStrategyRelabelSpec `json:"relabel,omitempty"`
}

type HandoverStrategyType string

const (
	// Relabel will change a specified label object after object.
	HandoverStrategyRelabel HandoverStrategyType = "Relabel"
)

// Relabel handover strategy definition.
type HandoverStrategyRelabelSpec struct {
	// LabelKey defines the labelKey to change the value of.
	// +kubebuilder:validation:MinLength=1
	LabelKey string `json:"labelKey"`

	// FromValue defines the initial value of the label.
	// +kubebuilder:validation:MinLength=1
	FromValue string `json:"fromValue"`

	// ToValue defines the desired value of the label after handover.
	// +kubebuilder:validation:MinLength=1
	ToValue string `json:"toValue"`

	// Status path to validate that the new operator is posting status information now.
	StatusPath string `json:"statusPath"`

	// MaxUnavailable defines how many objects may become unavailable due to the handover at the same time.
	// Cannot be below 1, because we cannot surge while relabling to create more instances.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	MaxUnavailable int `json:"maxUnavailable"`
}

// HandoverStatus defines the observed state of a Handover
type HandoverStatus struct {
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
	UID       types.UID `json:"uid"`
	Name      string    `json:"name"`
	Namespace string    `json:"namespace,omitempty"`
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
