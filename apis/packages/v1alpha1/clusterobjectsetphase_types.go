package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ClusterObjectSetPhaseSpec defines the desired state of a ClusterObjectSetPhase.
type ClusterObjectSetPhaseSpec struct {
	// Paused disables reconcilation of the ClusterObjectSetPhase,
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

// ClusterObjectSetPhaseStatus defines the observed state of a ClusterObjectSetPhase
type ClusterObjectSetPhaseStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// List of objects, the controller has paused reconcilation on.
	PausedFor []ObjectSetPausedObject `json:"pausedFor,omitempty"`
}

// ClusterObjectSetPhase is the Schema for the ClusterObjectSetPhases API
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterObjectSetPhase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterObjectSetPhaseSpec   `json:"spec,omitempty"`
	Status ClusterObjectSetPhaseStatus `json:"status,omitempty"`
}

// ClusterObjectSetPhaseList contains a list of ClusterObjectSetPhases
// +kubebuilder:object:root=true
type ClusterObjectSetPhaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterObjectSetPhase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterObjectSetPhase{}, &ClusterObjectSetPhaseList{})
}
