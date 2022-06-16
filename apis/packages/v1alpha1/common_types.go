package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

// ObjectSet Condition Types
const (
	ObjectSetAvailable = "Available"
	ObjectSetPaused    = "Paused"
	ObjectSetArchived  = "Archived"
	// Succeeded condition is only set once,
	// after a ObjectSet became Available for the first time.
	ObjectSetSucceeded = "Succeeded"
)

type ObjectSetPhase string

// Well-known ObjectSet Phases for printing a Status in kubectl,
// see deprecation notice in ObjectSetStatus for details.
const (
	ObjectSetPhasePending           ObjectSetPhase = "Pending"
	ObjectSetPhaseAvailable         ObjectSetPhase = "Available"
	ObjectSetPhaseNotReady          ObjectSetPhase = "NotReady"
	ObjectSetPhaseMissingDependency ObjectSetPhase = "MissingDependency"
	ObjectSetPhaseArchived          ObjectSetPhase = "Archived"
)

// ObjectDeployment Condition Types
const (
	ObjectDeploymentAvailable   = "Available"
	ObjectDeploymentProgressing = "Progressing"
)

type ObjectDeploymentPhase string

// Well-known ObjectDeployment Phases for printing a Status in kubectl,
// see deprecation notice in ObjectDeploymentStatus for details.
const (
	ObjectDeploymentPhasePending     ObjectDeploymentPhase = "Pending"
	ObjectDeploymentPhaseAvailable   ObjectDeploymentPhase = "Available"
	ObjectDeploymentPhaseNotReady    ObjectDeploymentPhase = "NotReady"
	ObjectDeploymentPhaseProgressing ObjectDeploymentPhase = "Progressing"
)

// ObjectSet specification.
type ObjectSetTemplateSpec struct {
	// Reconcile phase configuration for a ObjectSet.
	// Objects in each phase will be reconciled in order and checked with
	// given ReadinessProbes before continuing with the next phase.
	Phases []ObjectSetPhaseSpec `json:"phases"`
	// Readiness Probes check objects that are part of the package.
	// All probes need to succeed for a package to be considered Available.
	// Failing probes will prevent the reconcilation of objects in later phases.
	ReadinessProbes []ObjectSetProbe      `json:"readinessProbes"`
	Dependencies    []ObjectSetDependency `json:"dependencies,omitempty"`
}

// Specifies that the reconcilation of a specific object should be paused.
type ObjectSetPausedObject struct {
	// Object Kind.
	Kind string `json:"kind"`
	// Object Group.
	Group string `json:"group"`
	// Object Name.
	Name string `json:"name"`
}

// ObjectSet reconcile phase.
type ObjectSetPhaseSpec struct {
	// Name of the reconcile phase.
	Name string `json:"name"`
	// Objects belonging to this phase.
	Objects []ObjectSetObject `json:"objects"`
}

// An object that is part of an ObjectSet.
type ObjectSetObject struct {
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	Object runtime.RawExtension `json:"object"`
}

// ObjectSetProbe define how ObjectSets check their children for their status.
type ObjectSetProbe struct {
	// Probe configuration parameters.
	Probes []Probe `json:"probes"`
	// Selector specifies which objects this probe should target.
	Selector ProbeSelector `json:"selector"`
}

type ProbeSelectorType string

const (
	ProbeSelectorKind ProbeSelectorType = "Kind"
)

type ProbeSelector struct {
	// Type of the package probe.
	// +kubebuilder:validation:Enum=Kind
	Type ProbeSelectorType `json:"type"`
	// Kind specific configuration parameters. Only present if Type = Kind.
	Kind *PackageProbeKindSpec `json:"kind,omitempty"`
}

// Kind package probe parameters.
type PackageProbeKindSpec struct {
	// Object Group to apply a probe to.
	Group string `json:"group"`
	// Object Kind to apply a probe to.
	Kind string `json:"kind"`
}

// Defines probe parameters to check parts of a package.
type Probe struct {
	// Type of the probe.
	// +kubebuilder:validation:Enum=Condition;FieldsEqual
	Type ProbeType `json:"type"`
	// Condition specific configuration parameters. Only present if Type = Condition.
	Condition   *ProbeConditionSpec   `json:"condition,omitempty"`
	FieldsEqual *ProbeFieldsEqualSpec `json:"fieldsEqual,omitempty"`
}

type ProbeType string

const (
	ProbeCondition   ProbeType = "Condition"
	ProbeFieldsEqual ProbeType = "FieldsEqual"
)

// Condition Probe parameters.
type ProbeConditionSpec struct {
	// Condition Type to probe for.
	Type string `json:"type"`
	// Condition status to probe for.
	// +kubebuilder:default="True"
	Status string `json:"status"`
}

// Compares two fields specified by JSON Paths.
type ProbeFieldsEqualSpec struct {
	FieldA string `json:"fieldA"`
	FieldB string `json:"fieldB"`
}

// Package dependency describes prequesites of a package,
// that need to be met prior to installation.
type ObjectSetDependency struct {
	Type          ObjectSetDependencyType               `json:"type"`
	KubernetesAPI *ObjectSetDependencyKubernetesAPISpec `json:"kubernetesAPI,omitempty"`
}

type ObjectSetDependencyType string

const (
	// Declares to depend on a certain Kubernetes API.
	ObjectSetDependencyKubernetesAPI ObjectSetDependencyType = "KubernetesAPI"
)

// KubernetesAPI Dependency parameters.
type ObjectSetDependencyKubernetesAPISpec struct {
	// Group of the API.
	Group string `json:"group,omitempty"`
	// Version of the API.
	Version string `json:"version"`
	// Kind of the API.
	Kind string `json:"kind"`
}
