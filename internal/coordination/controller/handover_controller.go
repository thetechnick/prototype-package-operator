package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	"github.com/thetechnick/package-operator/internal/dynamicwatcher"
	internalprobe "github.com/thetechnick/package-operator/internal/probe"
)

type HandoverReconciler struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	DynamicClient   dynamic.Interface
	DiscoveryClient *discovery.DiscoveryClient

	dw *dynamicwatcher.DynamicWatcher
}

func (r *HandoverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.dw = dynamicwatcher.New(r.Log, r.Scheme, r.RESTMapper(), r.DynamicClient)

	return ctrl.NewControllerManagedBy(mgr).
		For(&coordinationv1alpha1.Handover{}).
		Watches(r.dw, &dynamicwatcher.EnqueueWatchingObjects{
			WatcherType:      &coordinationv1alpha1.Handover{},
			WatcherRefGetter: r.dw,
		}).
		Complete(r)
}

func (r *HandoverReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("Handover", req.NamespacedName.String())

	handover := &coordinationv1alpha1.Handover{}
	if err := r.Get(ctx, req.NamespacedName, handover); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !handover.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, handleDeletion(ctx, r.Client, r.dw, handover)
	}

	// Add finalizers
	if err := ensureCacheFinalizer(ctx, r.Client, handover); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure watch
	gvk, objType, objListType := unstructuredFromTargetAPI(handover.Spec.TargetAPI)
	if err := r.dw.Watch(handover, objType); err != nil {
		return ctrl.Result{}, fmt.Errorf("watching %s: %w", gvk, err)
	}

	// Parse Probes
	probe := internalprobe.Parse(handover.Spec.Probes)

	// Handle processing objects
	var err error
	if handover.Status.Processing, err = handleProcessing(
		ctx, r.Client, log, handover.Spec.Strategy, objType, probe, handover.Status.Processing); err != nil {
		return ctrl.Result{}, err
	}

	// List all objects
	// select all objects with new or old label value
	requirement, err := labels.NewRequirement(
		handover.Spec.Strategy.Relabel.LabelKey,
		selection.In,
		[]string{
			handover.Spec.Strategy.Relabel.ToValue,
			handover.Spec.Strategy.Relabel.FromValue,
		},
	)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building selector: %w", err)
	}
	selector := labels.NewSelector().Add(*requirement)

	if err := r.List(
		ctx, objListType,
		client.InNamespace(handover.Namespace),
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing %s: %w", gvk, err)
	}

	// Check state
	var unavailable int
	for _, obj := range objListType.Items {
		if success, _ := probe.Probe(&obj); !success {
			unavailable++
		}
	}

	// split into old and new
	groups := groupByLabelValues(
		objListType.Items, handover.Spec.Strategy.Relabel.LabelKey,
		handover.Spec.Strategy.Relabel.ToValue,
		handover.Spec.Strategy.Relabel.FromValue,
	)
	newObjs := groups[0]
	oldObjs := groups[1]

	for _, obj := range oldObjs {
		if len(handover.Status.Processing)+unavailable >= handover.
			Spec.Strategy.Relabel.MaxUnavailable {
			break
		}

		handover.Status.Processing = append(
			handover.Status.Processing,
			coordinationv1alpha1.HandoverRef{
				UID:       obj.GetUID(),
				Name:      obj.GetName(),
				Namespace: handover.Namespace,
			})
	}

	// report counts
	handover.Status.Stats.Found = int32(len(objListType.Items))
	handover.Status.Stats.Updated = int32(len(newObjs))
	handover.Status.Stats.Available = handover.Status.Stats.Found - int32(unavailable)

	handover.Status.ObservedGeneration = handover.Generation
	if handover.Status.Stats.Found == handover.Status.Stats.Updated && len(handover.Status.Processing) == 0 {
		handover.Status.Phase = coordinationv1alpha1.HandoverPhaseCompleted
		meta.SetStatusCondition(&handover.Status.Conditions, metav1.Condition{
			Type:               coordinationv1alpha1.HandoverCompleted,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: handover.Generation,
			Reason:             "Complete",
			Message:            "All found objects have been re-labeled.",
		})
	} else {
		handover.Status.Phase = coordinationv1alpha1.HandoverPhaseProgressing
		meta.SetStatusCondition(&handover.Status.Conditions, metav1.Condition{
			Type:               coordinationv1alpha1.HandoverCompleted,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: handover.Generation,
			Reason:             "Incomplete",
			Message:            "Some found objects need to be re-labeled.",
		})
	}

	if err := r.Status().Update(ctx, handover); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating Handover status: %w", err)
	}
	return ctrl.Result{}, nil
}

func handleProcessing(
	ctx context.Context,
	c client.Client,
	log logr.Logger,
	handoverStrategy coordinationv1alpha1.HandoverStrategy,
	objType *unstructured.Unstructured,
	probe internalprobe.Interface,
	processing []coordinationv1alpha1.HandoverRef,
) (stillProcessing []coordinationv1alpha1.HandoverRef, err error) {
	for _, processing := range processing {
		processingObj := objType.DeepCopy()
		err := c.Get(ctx, types.NamespacedName{
			Name:      processing.Name,
			Namespace: processing.Namespace,
		}, processingObj)
		if errors.IsNotFound(err) {
			// Object gone, remove it from processing queue.
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("getting object in process queue: %w", err)
		}

		labels := processingObj.GetLabels()
		if labels == nil ||
			labels[handoverStrategy.Relabel.LabelKey] != handoverStrategy.Relabel.ToValue {
			labels[handoverStrategy.Relabel.LabelKey] = handoverStrategy.Relabel.ToValue
			processingObj.SetLabels(labels)
			if err := c.Update(ctx, processingObj); err != nil {
				return nil, fmt.Errorf("updating object in process queue: %w", err)
			}
		}

		fieldPath := strings.Split(strings.Trim(handoverStrategy.Relabel.StatusPath, "."), ".")
		statusValue, ok, err := unstructured.NestedString(processingObj.Object, fieldPath...)
		if err != nil {
			return nil, fmt.Errorf("getting status value: %w", err)
		}
		if !ok || statusValue != handoverStrategy.Relabel.ToValue {
			log.Info("waiting for status field to update", "objName", processing.Name)
			stillProcessing = append(stillProcessing, processing)
			break
		}

		if success, message := probe.Probe(processingObj); !success {
			log.Info("waiting to be ready", "objName", processing.Name, "failure", message)
			stillProcessing = append(stillProcessing, processing)
		}
	}
	return stillProcessing, nil
}
