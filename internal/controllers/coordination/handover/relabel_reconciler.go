package handover

import (
	"context"
	"fmt"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers"
	"github.com/thetechnick/package-operator/internal/controllers/coordination"
	internalprobe "github.com/thetechnick/package-operator/internal/probe"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/util/jsonpath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type relabelReconciler struct {
	client client.Client
}

func (r *relabelReconciler) Reconcile(
	ctx context.Context, handover genericHandover) (ctrl.Result, error) {
	if handover.GetStrategyType() != genericStrategyRelabel {
		return ctrl.Result{}, nil
	}

	gvk, objType, objListType := coordination.UnstructuredFromTargetAPI(handover.GetTargetAPI())
	relabelSpec := handover.GetRelabelSpec()

	combinedProbe := internalprobe.Parse(handover.GetProbes())

	// Handle processing objects
	stillProcessing, err := r.handleAllProcessing(ctx, *relabelSpec,
		objType, combinedProbe, handover.GetProcessing())
	if err != nil {
		return ctrl.Result{}, err
	}
	handover.SetProcessing(stillProcessing)

	// List all objects
	// select all objects with new or old label value
	requirement, err := labels.NewRequirement(
		relabelSpec.LabelKey,
		selection.In,
		[]string{
			relabelSpec.ToValue,
			relabelSpec.FromValue,
		},
	)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building selector: %w", err)
	}
	selector := labels.NewSelector().Add(*requirement)

	if err := r.client.List(
		ctx, objListType,
		client.InNamespace(handover.ClientObject().GetNamespace()),
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing %s: %w", gvk, err)
	}

	// Check state
	var unavailable int
	for _, obj := range objListType.Items {
		if success, _ := combinedProbe.Probe(&obj); !success {
			unavailable++
		}
	}

	// split into old and new
	groups := groupByLabelValues(
		objListType.Items, relabelSpec.LabelKey,
		relabelSpec.ToValue,
		relabelSpec.FromValue,
	)
	newObjs := groups[0]
	oldObjs := groups[1]

	processing := handover.GetProcessing()
	for _, obj := range oldObjs {
		if len(processing)+unavailable >= relabelSpec.MaxUnavailable {
			break
		}

		// add a new item to the processing queue
		processing = append(
			processing,
			coordinationv1alpha1.HandoverRef{
				UID:       obj.GetUID(),
				Name:      obj.GetName(),
				Namespace: handover.ClientObject().GetNamespace(),
			})
	}
	handover.SetProcessing(processing)

	// report counts
	var stats coordinationv1alpha1.HandoverStatusStats
	stats.Found = int32(len(objListType.Items))
	stats.Updated = int32(len(newObjs))
	stats.Available = stats.Found - int32(unavailable)
	handover.SetStats(stats)

	if stats.Found == stats.Updated && len(processing) == 0 {
		meta.SetStatusCondition(handover.GetConditions(), metav1.Condition{
			Type:               coordinationv1alpha1.HandoverCompleted,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: handover.ClientObject().GetGeneration(),
			Reason:             "Complete",
			Message:            "All found objects have been re-labeled.",
		})
	} else {
		meta.SetStatusCondition(handover.GetConditions(), metav1.Condition{
			Type:               coordinationv1alpha1.HandoverCompleted,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: handover.ClientObject().GetGeneration(),
			Reason:             "Incomplete",
			Message:            "Some found objects need to be re-labeled.",
		})
	}

	if err := r.client.Status().Update(ctx, handover.ClientObject()); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating Handover status: %w", err)
	}
	return ctrl.Result{}, nil
}

func (r *relabelReconciler) handleAllProcessing(
	ctx context.Context,
	relabelSpec coordinationv1alpha1.HandoverStrategyRelabelSpec,
	objType *unstructured.Unstructured,
	probe internalprobe.Interface,
	processing []coordinationv1alpha1.HandoverRef,
) (stillProcessing []coordinationv1alpha1.HandoverRef, err error) {
	for _, handoverRef := range processing {
		finished, err := r.handleSingleProcessing(
			ctx, relabelSpec, objType, probe, handoverRef)
		if err != nil {
			return stillProcessing, err
		}
		if !finished {
			stillProcessing = append(stillProcessing, handoverRef)
		}
	}
	return stillProcessing, nil
}

func (r *relabelReconciler) handleSingleProcessing(
	ctx context.Context,
	relabelSpec coordinationv1alpha1.HandoverStrategyRelabelSpec,
	objType *unstructured.Unstructured,
	probe internalprobe.Interface,
	handoverRef coordinationv1alpha1.HandoverRef,
) (finished bool, err error) {
	log := controllers.LoggerFromContext(ctx)

	processingObj := objType.DeepCopy()
	err = r.client.Get(ctx, client.ObjectKey{
		Name:      handoverRef.Name,
		Namespace: handoverRef.Namespace,
	}, processingObj)
	if errors.IsNotFound(err) {
		// Object gone, remove it from processing queue.
		finished = true
		err = nil
		return
	}
	if err != nil {
		return false, fmt.Errorf("getting object in process queue: %w", err)
	}

	// Relabel Strategy
	labels := processingObj.GetLabels()
	if labels == nil ||
		labels[relabelSpec.LabelKey] != relabelSpec.ToValue {
		labels[relabelSpec.LabelKey] = relabelSpec.ToValue
		processingObj.SetLabels(labels)
		if err := r.client.Update(ctx, processingObj); err != nil {
			return false, fmt.Errorf("updating object in process queue: %w", err)
		}
	}

	jsonPath := jsonpath.New("status-thing!!!").AllowMissingKeys(true)
	// TODO: SOOOO much validation for paths
	if err := jsonPath.Parse("{" + relabelSpec.StatusPath + "}"); err != nil {
		return false, fmt.Errorf("invalid jsonpath: %w", err)
	}

	statusValues, err := jsonPath.FindResults(processingObj.Object)
	if err != nil {
		return false, fmt.Errorf("getting status value: %w", err)
	}

	// TODO: even more proper handling
	if len(statusValues[0]) > 1 {
		return false, fmt.Errorf("multiple status values returned: %s", statusValues)
	}
	if len(statusValues[0]) == 0 {
		// no reported status
		return false, nil
	}

	statusValue := statusValues[0][0].Interface()
	if statusValue != relabelSpec.ToValue {
		log.Info("waiting for status field to update", "objName", handoverRef.Name)
		return false, nil
	}

	if success, message := probe.Probe(processingObj); !success {
		log.Info("waiting to be ready", "objName", handoverRef.Name, "failure", message)
		return false, nil
	}

	return true, nil
}

// given a list of objects this function will group all objects with the same label value.
// the return slice is garanteed to be of the same size as the amount of values given to the function.
func groupByLabelValues(in []unstructured.Unstructured, labelKey string, values ...string) [][]unstructured.Unstructured {
	out := make([][]unstructured.Unstructured, len(values))
	for _, obj := range in {
		if obj.GetLabels() == nil {
			continue
		}
		if len(obj.GetLabels()[labelKey]) == 0 {
			continue
		}

		for i, v := range values {
			if obj.GetLabels()[labelKey] == v {
				out[i] = append(out[i], obj)
			}
		}
	}
	return out
}
