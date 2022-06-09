package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterPackageDeploymentSpec defines the desired state of a ClusterPackageDeployment.
type ClusterPackageDeploymentSpec struct {
	// Number of old revisions in the form of archived PackageSets to keep.
	// +kubebuilder:default=5
	RevisionHistoryLimit *int `json:"revisionHistoryLimit,omitempty"`
	// Selector targets PackageSets managed by this Deployment.
	Selector metav1.LabelSelector `json:"selector"`
	// Template to create new PackageSets from.
	Template PackageSetTemplate `json:"template"`
}

// ClusterPackageDeploymentStatus defines the observed state of a ClusterPackageDeployment
type ClusterPackageDeploymentStatus struct {
	// The most recent generation observed by the controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions is a list of status conditions ths object is in.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// DEPRECATED: This field is not part of any API contract
	// it will go away as soon as kubectl can print conditions!
	// Human readable status - please use .Conditions from code
	Phase ClusterPackageDeploymentPhase `json:"phase,omitempty"`
	// Count of hash collisions of the ClusterPackageDeployment.
	CollisionCount *int32 `json:"collisionCount,omitempty"`
	// Computed TemplateHash.
	TemplateHash string `json:"templateHash,omitempty"`
}

const (
	ClusterPackageDeploymentAvailable   = "Available"
	ClusterPackageDeploymentProgressing = "Progressing"
)

type ClusterPackageDeploymentPhase string

// Well-known ClusterPackageDeployment Phases for printing a Status in kubectl,
// see deprecation notice in ClusterPackageDeploymentStatus for details.
const (
	ClusterPackageDeploymentPhasePending     ClusterPackageDeploymentPhase = "Pending"
	ClusterPackageDeploymentPhaseAvailable   ClusterPackageDeploymentPhase = "Available"
	ClusterPackageDeploymentPhaseNotReady    ClusterPackageDeploymentPhase = "NotReady"
	ClusterPackageDeploymentPhaseProgressing ClusterPackageDeploymentPhase = "Progressing"
)

// ClusterPackageDeployment is the Schema for the ClusterPackageDeployments API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterPackageDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterPackageDeploymentSpec `json:"spec,omitempty"`
	// +kubebuilder:default={phase:Pending}
	Status ClusterPackageDeploymentStatus `json:"status,omitempty"`
}

// ClusterPackageDeploymentList contains a list of ClusterPackageDeployments
// +kubebuilder:object:root=true
type ClusterPackageDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterPackageDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterPackageDeployment{}, &ClusterPackageDeploymentList{})
}
