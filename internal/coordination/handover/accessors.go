package handover

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	internalprobe "github.com/thetechnick/package-operator/internal/probe"
)

func getTargetAPI(handover client.Object) coordinationv1alpha1.TargetAPI {
	switch o := handover.(type) {
	case *coordinationv1alpha1.Handover:
		return o.Spec.TargetAPI
	case *coordinationv1alpha1.ClusterHandover:
		return o.Spec.TargetAPI
	}
	panic("invalid Handover object")
}

func parseProbes(handover client.Object) internalprobe.Interface {
	switch o := handover.(type) {
	case *coordinationv1alpha1.Handover:
		return internalprobe.Parse(o.Spec.Probes)
	case *coordinationv1alpha1.ClusterHandover:
		return internalprobe.Parse(o.Spec.Probes)
	}
	panic("invalid Handover object")
}

func getStrategy(handover client.Object) coordinationv1alpha1.HandoverStrategy {
	switch o := handover.(type) {
	case *coordinationv1alpha1.Handover:
		return o.Spec.Strategy
	case *coordinationv1alpha1.ClusterHandover:
		return o.Spec.Strategy
	}
	panic("invalid Handover object")
}

func getProcessing(handover client.Object) []coordinationv1alpha1.HandoverRef {
	switch o := handover.(type) {
	case *coordinationv1alpha1.Handover:
		return o.Status.Processing
	case *coordinationv1alpha1.ClusterHandover:
		return o.Status.Processing
	}
	panic("invalid Handover object")
}

func setProcessing(handover client.Object, processing []coordinationv1alpha1.HandoverRef) {
	switch o := handover.(type) {
	case *coordinationv1alpha1.Handover:
		o.Status.Processing = processing
	case *coordinationv1alpha1.ClusterHandover:
		o.Status.Processing = processing
	}
	panic("invalid Handover object")
}

func getStats(handover client.Object) coordinationv1alpha1.HandoverStatusStats {
	switch o := handover.(type) {
	case *coordinationv1alpha1.Handover:
		return o.Status.Stats
	case *coordinationv1alpha1.ClusterHandover:
		return o.Status.Stats
	}
	panic("invalid Handover object")
}

func setStats(handover client.Object, stats coordinationv1alpha1.HandoverStatusStats) {
	switch o := handover.(type) {
	case *coordinationv1alpha1.Handover:
		o.Status.Stats = stats
	case *coordinationv1alpha1.ClusterHandover:
		o.Status.Stats = stats
	}
	panic("invalid Handover object")
}

func setStatus(handover client.Object, conditions ...metav1.Condition) {
	var conds *[]metav1.Condition
	switch o := handover.(type) {
	case *coordinationv1alpha1.Handover:
		o.Status.ObservedGeneration = o.Generation
		conds = &o.Status.Conditions
		// o.Status.Phase = coordinationv1alpha1.HandoverPhaseActive

	case *coordinationv1alpha1.ClusterHandover:
		o.Status.ObservedGeneration = o.Generation
		conds = &o.Status.Conditions
		// o.Status.Phase = coordinationv1alpha1.ClusterHandoverPhaseActive

	default:
		panic("invalid Handover object")
	}

	var phase string
	for _, c := range conditions {
		// handover.Status.Phase = coordinationv1alpha1.HandoverPhaseCompleted
		if c.Type == coordinationv1alpha1.HandoverCompleted &&
			c.Status == metav1.ConditionTrue {
			phase = string(coordinationv1alpha1.HandoverPhaseCompleted)
		}

		meta.SetStatusCondition(conds, c)
	}
	if len(phase) == 0 {
		phase = string(coordinationv1alpha1.HandoverPhaseProgressing)
	}

	switch o := handover.(type) {
	case *coordinationv1alpha1.Handover:
		o.Status.Phase = coordinationv1alpha1.HandoverPhase(phase)

	case *coordinationv1alpha1.ClusterHandover:
		o.Status.Phase = coordinationv1alpha1.ClusterHandoverPhase(phase)
	}
}
