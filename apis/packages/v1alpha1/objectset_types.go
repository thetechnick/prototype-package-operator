package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ObjectSetSpec defines the desired state of a ObjectSet.
type ObjectSetSpec struct {
	// Archived will delete all unpaused objects and remove o
	Archived bool `json:"archived,omitempty"`
	// Paused disables reconcilation of the ObjectSet,
	// only Status updates will be propagated.
	Paused bool `json:"paused,omitempty"`
	// Pause reconcilation of specific objects.
	PausedFor []ObjectSetPausedObject `json:"pausedFor,omitempty"`

	// Immutable fields below
	ObjectSetTemplateSpec `json:",inline"`
}

// ObjectSetStatus defines the observed state of a ObjectSet
type ObjectSetStatus struct {
	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// DEPRECATED: This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// Human readable status - please use .Conditions from code
	Phase ObjectSetPhase `json:"phase,omitempty"`
	// List of objects, the controller has paused reconcilation on.
	PausedFor []ObjectSetPausedObject `json:"pausedFor,omitempty"`
}

// ObjectSet is the Schema for the ObjectSets API
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
