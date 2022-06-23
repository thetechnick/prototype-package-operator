package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

// ObjectSet specification.
type ObjectSetTemplateSpec struct {
	// Reconcile phase configuration for a ObjectSet.
	// Objects in each phase will be reconciled in order and checked with
	// given ReadinessProbes before continuing with the next phase.
	Phases []ObjectPhase `json:"phases"`
	// Readiness Probes check objects that are part of the package.
	// All probes need to succeed for a package to be considered Available.
	// Failing probes will prevent the reconcilation of objects in later phases.
	ReadinessProbes []ObjectSetProbe `json:"readinessProbes"`
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
type ObjectPhase struct {
	// Name of the reconcile phase.
	Name string `json:"name"`
	// Class of the underlying phase controller.
	Class string `json:"class,omitempty"`
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
