package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ObjectSetPhaseSpec defines the desired state of a ObjectSetPhase.
type ObjectSetPhaseSpec struct {
	// Paused disables reconcilation of the ObjectSetPhase,
	// only Status updates will be propagated.
	Paused bool `json:"paused,omitempty"`
	// Pause reconcilation of specific objects.
	PausedFor []ObjectSetPausedObject `json:"pausedFor,omitempty"`

	// Readiness Probes check objects that are part of the package.
	// All probes need to succeed for a package to be considered Available.
	// Failing probes will prevent the reconcilation of objects in later phases.
	ReadinessProbes []ObjectSetProbe `json:"readinessProbes"`

	// Immutable fields below
	ObjectPhase `json:",inline"`
}

// ObjectSetPhaseStatus defines the observed state of a ObjectSetPhase
type ObjectSetPhaseStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// List of objects, the controller has paused reconcilation on.
	PausedFor []ObjectSetPausedObject `json:"pausedFor,omitempty"`
}

// ObjectSetPhase is the Schema for the ObjectSetPhases API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ObjectSetPhase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ObjectSetPhaseSpec   `json:"spec,omitempty"`
	Status ObjectSetPhaseStatus `json:"status,omitempty"`
}

// ObjectSetPhaseList contains a list of ObjectSetPhases
// +kubebuilder:object:root=true
type ObjectSetPhaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ObjectSetPhase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ObjectSetPhase{}, &ObjectSetPhaseList{})
}
