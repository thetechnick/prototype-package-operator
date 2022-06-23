package handover

import (
	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type genericStrategyType string

const (
	genericStrategyRelabel genericStrategyType = "Relabel"
)

type genericHandover interface {
	ClientObject() client.Object
	UpdatePhase()
	GetTargetAPI() coordinationv1alpha1.TargetAPI
	GetConditions() *[]metav1.Condition
	GetProbes() []packagesv1alpha1.Probe
	GetStrategyType() genericStrategyType
	GetProcessing() []coordinationv1alpha1.HandoverRef
	GetRelabelSpec() *coordinationv1alpha1.HandoverStrategyRelabelSpec
	SetProcessing(processing []coordinationv1alpha1.HandoverRef)
	SetStats(stats coordinationv1alpha1.HandoverStatusStats)
}

var (
	_ genericHandover = (*GenericHandover)(nil)
	_ genericHandover = (*GenericClusterHandover)(nil)
)

type GenericHandover struct {
	coordinationv1alpha1.Handover
}

func (a *GenericHandover) ClientObject() client.Object {
	return &a.Handover
}

func (a *GenericHandover) UpdatePhase() {
	if meta.IsStatusConditionTrue(
		a.Status.Conditions, coordinationv1alpha1.HandoverCompleted) {
		a.Status.Phase = coordinationv1alpha1.HandoverPhaseCompleted
	} else {
		a.Status.Phase = coordinationv1alpha1.HandoverPhaseProgressing
	}
}

func (a *GenericHandover) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericHandover) GetTargetAPI() coordinationv1alpha1.TargetAPI {
	return a.Spec.TargetAPI
}

func (a *GenericHandover) GetProbes() []packagesv1alpha1.Probe {
	return a.Spec.Probes
}

func (a *GenericHandover) GetStrategyType() genericStrategyType {
	return genericStrategyType(a.Spec.Strategy.Type)
}

func (a *GenericHandover) GetProcessing() []coordinationv1alpha1.HandoverRef {
	return a.Status.Processing
}
func (a *GenericHandover) SetProcessing(processing []coordinationv1alpha1.HandoverRef) {
	a.Status.Processing = processing
}

func (a *GenericHandover) GetRelabelSpec() *coordinationv1alpha1.HandoverStrategyRelabelSpec {
	return a.Spec.Strategy.Relabel
}

func (a *GenericHandover) SetStats(stats coordinationv1alpha1.HandoverStatusStats) {
	a.Status.Stats = stats
}

type GenericClusterHandover struct {
	coordinationv1alpha1.ClusterHandover
}

func (a *GenericClusterHandover) ClientObject() client.Object {
	return &a.ClusterHandover
}

func (a *GenericClusterHandover) UpdatePhase() {
	if meta.IsStatusConditionTrue(
		a.Status.Conditions, coordinationv1alpha1.HandoverCompleted) {
		a.Status.Phase = coordinationv1alpha1.ClusterHandoverPhaseCompleted
	} else {
		a.Status.Phase = coordinationv1alpha1.ClusterHandoverPhaseProgressing
	}
}

func (a *GenericClusterHandover) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericClusterHandover) GetTargetAPI() coordinationv1alpha1.TargetAPI {
	return a.Spec.TargetAPI
}

func (a *GenericClusterHandover) GetProbes() []packagesv1alpha1.Probe {
	return a.Spec.Probes
}

func (a *GenericClusterHandover) GetStrategyType() genericStrategyType {
	return genericStrategyType(a.Spec.Strategy.Type)
}

func (a *GenericClusterHandover) GetProcessing() []coordinationv1alpha1.HandoverRef {
	return a.Status.Processing
}
func (a *GenericClusterHandover) SetProcessing(processing []coordinationv1alpha1.HandoverRef) {
	a.Status.Processing = processing
}

func (a *GenericClusterHandover) GetRelabelSpec() *coordinationv1alpha1.HandoverStrategyRelabelSpec {
	return a.Spec.Strategy.Relabel
}

func (a *GenericClusterHandover) SetStats(stats coordinationv1alpha1.HandoverStatusStats) {
	a.Status.Stats = stats
}
