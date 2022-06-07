package adoption

import (
	"context"
	"fmt"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RoundRobinAdoptionReconciler[T operandPtr[O], O operand] struct {
	client client.Client
}

func (r *RoundRobinAdoptionReconciler[T, O]) Reconcile(
	ctx context.Context, adoption T) (ctrl.Result, error) {
	if getStrategyType(adoption) != roundRobinStrategy {
		setRoundRobinStatus(adoption, nil)

		// noop, a different strategy will match
		return ctrl.Result{}, nil
	}

	roundRobinSpec := getRoundRobinSpec(adoption)
	if roundRobinSpec == nil {
		// TODO: Validate via webhook
		return ctrl.Result{}, nil
	}

	selector, err := roundRobinNegativeLabelSelector(roundRobinSpec)
	if err != nil {
		return ctrl.Result{}, err
	}

	gvk, _, objListType := unstructuredFromTargetAPI(getTargetAPI(adoption))

	// List all the things not yet labeled
	if err := r.client.List(
		ctx, objListType,
		client.InNamespace(adoption.GetNamespace()), // can also set this for ClusterAdoption without issue.
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing %s: %w", gvk, err)
	}

	var rrIndex int
	for _, obj := range objListType.Items {
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		for k, v := range roundRobinSpec.Always {
			labels[k] = v
		}

		// choose an option
		lastIndex := getLastRoundRobinIndex(adoption)
		rrIndex = roundRobinIndex(lastIndex, len(roundRobinSpec.Options)-1)
		for k, v := range roundRobinSpec.Options[rrIndex] {
			labels[k] = v
		}

		obj.SetLabels(labels)

		if err := r.client.Update(ctx, &obj); err != nil {
			// try to save the last committed index
			setRoundRobinStatus(adoption, &coordinationv1alpha1.AdoptionRoundRobinStatus{
				LastIndex: lastIndex,
			})
			_ = r.client.Status().Update(ctx, adoption)
			return ctrl.Result{}, fmt.Errorf("setting labels: %w", err)
		}

		// track last committed index
		setRoundRobinStatus(adoption, &coordinationv1alpha1.AdoptionRoundRobinStatus{
			LastIndex: rrIndex,
		})
	}

	return ctrl.Result{}, nil
}

func roundRobinIndex(lastIndex int, max int) int {
	index := lastIndex + 1
	if index > max {
		return 0
	}
	return index
}

func setRoundRobinStatus(
	adoption client.Object,
	rr *coordinationv1alpha1.AdoptionRoundRobinStatus,
) {
	switch o := adoption.(type) {
	case *coordinationv1alpha1.Adoption:
		o.Status.RoundRobin = rr
	case *coordinationv1alpha1.ClusterAdoption:
		o.Status.RoundRobin = rr
	}
}

func getLastRoundRobinIndex(adoption client.Object) int {
	switch o := adoption.(type) {
	case *coordinationv1alpha1.Adoption:
		if o.Status.RoundRobin != nil {
			return o.Status.RoundRobin.LastIndex
		}
	case *coordinationv1alpha1.ClusterAdoption:
		if o.Status.RoundRobin != nil {
			return o.Status.RoundRobin.LastIndex
		}
	}
	return -1 // no last index
}

func getRoundRobinSpec(adoption client.Object) *coordinationv1alpha1.AdoptionStrategyRoundRobinSpec {
	switch o := adoption.(type) {
	case *coordinationv1alpha1.Adoption:
		return o.Spec.Strategy.RoundRobin
	case *coordinationv1alpha1.ClusterAdoption:
		return o.Spec.Strategy.RoundRobin
	}
	return nil
}

// Builds a labelSelector EXCLUDING all objects that could be targeted.
func roundRobinNegativeLabelSelector(
	roundRobin *coordinationv1alpha1.AdoptionStrategyRoundRobinSpec,
) (labels.Selector, error) {
	var requirements []labels.Requirement

	// Commented out:
	// If any of the "always"-labels already exists,
	// it has no effect on the round robin distribution.
	//
	// for k := range roundRobin.Always {
	// 	requirement, err := labels.NewRequirement(
	// 		k, selection.DoesNotExist, nil)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("building requirement: %w", err)
	// 	}
	// 	requirements = append(requirements, *requirement)
	// }

	for i := range roundRobin.Options {
		for k := range roundRobin.Options[i] {
			requirement, err := labels.NewRequirement(
				k, selection.DoesNotExist, nil)
			if err != nil {
				return nil, fmt.Errorf("building requirement: %w", err)
			}
			requirements = append(requirements, *requirement)
		}
	}

	selector := labels.NewSelector().Add(requirements...)
	return selector, nil
}
