package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PackageDeploymentSpec defines the desired state of a PackageDeployment.
type PackageDeploymentSpec struct {
	// Number of old revisions in the form of archived PackageSets to keep.
	// +kubebuilder:default=5
	RevisionHistoryLimit *int `json:"revisionHistoryLimit,omitempty"`
	// Selector targets PackageSets managed by this Deployment.
	Selector metav1.LabelSelector `json:"selector"`
	// Template to create new PackageSets from.
	Template PackageSetTemplate `json:"template"`
}

// PackageSetTemplate describes the template to create new PackageSets from.
type PackageSetTemplate struct {
	// Common Object Metadata.
	Metadata metav1.ObjectMeta `json:"metadata"`
	// PackageSet specification.
	Spec PackageSetTemplateSpec `json:"spec"`
}

// PackageDeploymentStatus defines the observed state of a PackageDeployment
type PackageDeploymentStatus struct {
	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// DEPRECATED: This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// Human readable status - please use .Conditions from code
	Phase PackageDeploymentPhase `json:"phase,omitempty"`
	// Count of hash collisions of the PackageDeployment.
	CollisionCount *int32 `json:"collisionCount,omitempty"`
	// Computed TemplateHash.
	TemplateHash string `json:"templateHash,omitempty"`
}

const (
	PackageDeploymentAvailable   = "Available"
	PackageDeploymentProgressing = "Progressing"
)

type PackageDeploymentPhase string

// Well-known PackageDeployment Phases for printing a Status in kubectl,
// see deprecation notice in PackageDeploymentStatus for details.
const (
	PackageDeploymentPhasePending     PackageDeploymentPhase = "Pending"
	PackageDeploymentPhaseAvailable   PackageDeploymentPhase = "Available"
	PackageDeploymentPhaseNotReady    PackageDeploymentPhase = "NotReady"
	PackageDeploymentPhaseProgressing PackageDeploymentPhase = "Progressing"
)

// PackageDeployment is the Schema for the PackageDeployments API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type PackageDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PackageDeploymentSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status PackageDeploymentStatus `json:"status,omitempty"`
}

// PackageDeploymentList contains a list of PackageDeployments
// +kubebuilder:object:root=true
type PackageDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PackageDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PackageDeployment{}, &PackageDeploymentList{})
}
