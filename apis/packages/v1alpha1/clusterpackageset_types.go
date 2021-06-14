package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterPackageSetSpec defines the desired state of a ClusterPackageSet.
type ClusterPackageSetSpec struct {
	// Archived will delete all unpaused objects and remove o
	Archived bool `json:"archived,omitempty"`
	// Paused disables reconcilation of the ClusterPackageSet,
	// only Status updates will be propagated.
	Paused bool `json:"paused,omitempty"`
	// Pause reconcilation of specific objects.
	PausedFor []PackagePausedObject `json:"pausedFor,omitempty"`

	// Immutable fields below
	PackageSetTemplateSpec `json:",inline"`
}

// ClusterPackageSetStatus defines the observed state of a ClusterPackageSet
type ClusterPackageSetStatus struct {
	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// DEPRECATED: This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// Human readable status - please use .Conditions from code
	Phase ClusterPackageSetPhase `json:"phase,omitempty"`
	// List of objects, the controller has paused reconcilation on.
	PausedFor []PackagePausedObject `json:"pausedFor,omitempty"`
}

const (
	ClusterPackageSetAvailable = "Available"
	ClusterPackageSetPaused    = "Paused"
	ClusterPackageSetArchived  = "Archived"
	// Succeeded condition is only set once,
	// after a ClusterPackageSet became Available for the first time.
	ClusterPackageSetSucceeded = "Succeeded"
)

type ClusterPackageSetPhase string

// Well-known ClusterPackageSet Phases for printing a Status in kubectl,
// see deprecation notice in ClusterPackageSetStatus for details.
const (
	ClusterPackageSetPhasePending           ClusterPackageSetPhase = "Pending"
	ClusterPackageSetPhaseAvailable         ClusterPackageSetPhase = "Available"
	ClusterPackageSetPhaseNotReady          ClusterPackageSetPhase = "NotReady"
	ClusterPackageSetPhaseMissingDependency ClusterPackageSetPhase = "MissingDependency"
	ClusterPackageSetPhaseArchived          ClusterPackageSetPhase = "Archived"
)

// ClusterPackageSet is the Schema for the ClusterPackageSets API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterPackageSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterPackageSetSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status ClusterPackageSetStatus `json:"status,omitempty"`
}

// ClusterPackageSetList contains a list of ClusterPackageSets
// +kubebuilder:object:root=true
type ClusterPackageSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterPackageSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterPackageSet{}, &ClusterPackageSetList{})
}
