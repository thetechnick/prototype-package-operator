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

type StaticAdoptionReconciler[T operandPtr[O], O operand] struct {
	client client.Client
}

func (r *StaticAdoptionReconciler[T, O]) Reconcile(
	ctx context.Context, adoption T) (ctrl.Result, error) {
	if getStrategyType(adoption) != staticStrategy {
		// noop, a different strategy will match
		return ctrl.Result{}, nil
	}

	specLabels := getStrategyStaticLabels(adoption)
	selector, err := negativeLabelSelectorFromLabels(specLabels)
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

func getStrategyStaticLabels(adoption client.Object) map[string]string {
	switch o := adoption.(type) {
	case *coordinationv1alpha1.Adoption:
		return o.Spec.Strategy.Static.Labels
	case *coordinationv1alpha1.ClusterAdoption:
		return o.Spec.Strategy.Static.Labels
	}
	return nil
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
