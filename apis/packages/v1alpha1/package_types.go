package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// PackageSpec defines the desired state of a Package.
type PackageSpec struct {
	PackageSourceSpec `json:",inline"`
}

type PackageSourceType string

const (
	PackageSourceTypeImage PackageSourceType = "Image"
)

type PackageSourceSpec struct {
	// Package source type
	// +kubebuilder:validation:Enum=Image
	Type PackageSourceType `json:"type"`
	// Image registry address and tag to get the package contents from.
	Image *string `json:"image,omitempty"`
}

// PackageStatus defines the observed state of a Package
type PackageStatus struct {
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// DEPRECATED: This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// Human readable status - please use .Conditions from code
	Phase PackageStatusPhase `json:"phase,omitempty"`
	// Hash of the PackageSourceSpec, used to track whether a new unpack is needed.
	SourceHash string `json:"sourceHash,omitempty"`
}

// Package condition types
const (
	// A Packages "Available" condition tracks the availability of the underlying ObjectDeployment objects.
	// When the Package is reporting "Available" = True, it's expected that whatever the Package installs is up and operational.
	// Package "Availability" may change multiple times during it's lifecycle.
	PackageAvailable   = "Available"
	PackageProgressing = "Progressing"
	PackageUnpacked    = "Unpacked"
)

type PackageStatusPhase string

// Well-known Package Phases for printing a Status in kubectl,
// see deprecation notice in PackageStatus for details.
const (
	PackagePhasePending     PackageStatusPhase = "Pending"
	PackagePhaseAvailable   PackageStatusPhase = "Available"
	PackagePhaseProgressing PackageStatusPhase = "Progressing"
	PackagePhaseNotReady    PackageStatusPhase = "NotReady"
)

// Package is the Schema for the Packages API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Package struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PackageSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status PackageStatus `json:"status,omitempty"`
}

// PackageList contains a list of Packages
// +kubebuilder:object:root=true
type PackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Package `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Package{}, &PackageList{})
}
