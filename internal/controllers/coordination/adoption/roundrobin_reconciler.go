package adoption

import (
	"context"
	"fmt"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers/coordination"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RoundRobinAdoptionReconciler struct {
	client client.Client
}

func (r *RoundRobinAdoptionReconciler) Reconcile(
	ctx context.Context, adoption genericAdoption) (ctrl.Result, error) {
	if adoption.GetStrategyType() != genericStrategyRoundRobin {
		adoption.SetRoundRobinStatus(nil)

		// noop, a different strategy will match
		return ctrl.Result{}, nil
	}

	roundRobinSpec := adoption.GetRoundRobinStrategy()
	if roundRobinSpec == nil {
		// TODO: Validate via webhook
		return ctrl.Result{}, nil
	}

	selector, err := roundRobinNegativeLabelSelector(roundRobinSpec)
	if err != nil {
		return ctrl.Result{}, err
	}

	gvk, _, objListType := coordination.UnstructuredFromTargetAPI(adoption.GetTargetAPI())

	// List all the things not yet labeled
	if err := r.client.List(
		ctx, objListType,
		client.InNamespace(adoption.ClientObject().GetNamespace()), // can also set this for ClusterAdoption without issue.
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
			adoption.SetRoundRobinStatus(&coordinationv1alpha1.AdoptionRoundRobinStatus{
				LastIndex: lastIndex,
			})
			_ = r.client.Status().Update(ctx, adoption.ClientObject())
			return ctrl.Result{}, fmt.Errorf("setting labels: %w", err)
		}

		// track last committed index
		adoption.SetRoundRobinStatus(&coordinationv1alpha1.AdoptionRoundRobinStatus{
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

func getLastRoundRobinIndex(adoption genericAdoption) int {
	rr := adoption.GetRoundRobinStatus()
	if rr != nil {
		return rr.LastIndex
	}
	return -1 // no last index
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
