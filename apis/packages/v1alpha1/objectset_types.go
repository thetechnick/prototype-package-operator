package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ObjectSetSpec defines the desired state of a ObjectSet.
type ObjectSetSpec struct {
	// Specifies the lifecycle state of the ObjectSet.
	// +kubebuilder:default="Active"
	// +kubebuilder:validation:Enum=Active;Paused;Archived
	LifecycleState ObjectSetLifecycleState `json:"lifecycleState,omitempty"`
	// Pause reconcilation of specific objects, while still reporting status.
	PausedFor []ObjectSetPausedObject `json:"pausedFor,omitempty"`
	// Immutable fields below
	ObjectSetTemplateSpec `json:",inline"`
}

// Specifies the lifecycle state of the ObjectSet.
type ObjectSetLifecycleState string

const (
	// "Active" is the default lifecycle state.
	ObjectSetLifecycleStateActive ObjectSetLifecycleState = "Active"
	// "Paused" disables reconcilation of the ObjectSet.
	// Only Status updates will still propagated, but object changes will not be reconciled.
	ObjectSetLifecycleStatePaused ObjectSetLifecycleState = "Paused"
	// "Archived" disables reconcilation while also "scaling to zero",
	// which deletes all objects that are not excluded via the pausedFor property and
	// removes itself from the owner list of all other objects previously under management.
	ObjectSetLifecycleStateArchived ObjectSetLifecycleState = "Archived"
)

// ObjectSetStatus defines the observed state of a ObjectSet
type ObjectSetStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// DEPRECATED: This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// Human readable status - please use .Conditions from code
	Phase ObjectSetStatusPhase `json:"phase,omitempty"`
	// List of objects, the controller has paused reconcilation on.
	PausedFor []ObjectSetPausedObject `json:"pausedFor,omitempty"`
}

// ObjectSet Condition Types
const (
	ObjectSetAvailable = "Available"
	ObjectSetPaused    = "Paused"
	ObjectSetArchived  = "Archived"
	// Succeeded condition is only set once,
	// after a ObjectSet became Available for the first time.
	ObjectSetSucceeded = "Succeeded"
)

type ObjectSetStatusPhase string

// Well-known ObjectSet Phases for printing a Status in kubectl,
// see deprecation notice in ObjectSetStatus for details.
const (
	ObjectSetPhasePending   ObjectSetStatusPhase = "Pending"
	ObjectSetPhaseAvailable ObjectSetStatusPhase = "Available"
	ObjectSetPhaseNotReady  ObjectSetStatusPhase = "NotReady"
	ObjectSetPhaseArchived  ObjectSetStatusPhase = "Archived"
)

// ObjectSet reconcile a collection of objects across ordered phases and aggregate their status.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ObjectSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ObjectSetSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status ObjectSetStatus `json:"status,omitempty"`
}

// ObjectSetList contains a list of ObjectSets
// +kubebuilder:object:root=true
type ObjectSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ObjectSet{}, &ObjectSetList{})
}
