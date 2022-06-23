package adoption

import (
	"context"
	"fmt"

	"github.com/thetechnick/package-operator/internal/controllers/coordination"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StaticAdoptionReconciler struct {
	client client.Client
}

func (r *StaticAdoptionReconciler) Reconcile(
	ctx context.Context, adoption genericAdoption) (ctrl.Result, error) {
	if adoption.GetStrategyType() != genericStrategyStatic {
		// noop, a different strategy will match
		return ctrl.Result{}, nil
	}

	specLabels := adoption.GetStaticStrategy().Labels
	selector, err := negativeLabelSelectorFromLabels(specLabels)
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

	for _, obj := range objListType.Items {
		labels := obj.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		for k, v := range specLabels {
			labels[k] = v
		}
		obj.SetLabels(labels)

		if err := r.client.Update(ctx, &obj); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting labels: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func negativeLabelSelectorFromLabels(specLabels map[string]string) (labels.Selector, error) {
	// Build selector
	var requirements []labels.Requirement
	for k := range specLabels {
		requirement, err := labels.NewRequirement(
			k, selection.DoesNotExist, nil)
		if err != nil {
			return nil, fmt.Errorf("building selector: %w", err)
		}
		requirements = append(requirements, *requirement)
	}
	selector := labels.NewSelector().Add(requirements...)
	return selector, nil
}
