package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

// Package reconcile phase.
// Packages are reconciled
type PackagePhase struct {
	// Name of the reconcile phase.
	Name string `json:"name"`
	// Objects belonging to this phase.
	Objects []PackageObject `json:"objects"`
}

// An object that is part of a package.
type PackageObject struct {
	// +kubebuilder:validation:EmbeddedResource
	// +kubebuilder:pruning:PreserveUnknownFields
	Object runtime.RawExtension `json:"object"`
}

// Package probes define how packages are checked for their status.
type PackageProbe struct {
	// Name of the probe.
	Name string `json:"name"`
	// Probe configuration parameters.
	Probe Probe `json:"probe"`
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

type ProbeFieldsEqualSpec struct {
	FieldA string `json:"fieldA"`
	FieldB string `json:"fieldB"`
}
