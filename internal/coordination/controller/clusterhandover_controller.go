package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	"github.com/thetechnick/package-operator/internal/dynamicwatcher"
	internalprobe "github.com/thetechnick/package-operator/internal/probe"
)

type ClusterHandoverReconciler struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	DynamicClient   dynamic.Interface
	DiscoveryClient *discovery.DiscoveryClient

	dw *dynamicwatcher.DynamicWatcher
}

func (r *ClusterHandoverReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.dw = dynamicwatcher.New(r.Log, r.Scheme, r.RESTMapper(), r.DynamicClient)

	return ctrl.NewControllerManagedBy(mgr).
		For(&coordinationv1alpha1.ClusterHandover{}).
		Watches(r.dw, &dynamicwatcher.EnqueueWatchingObjects{
			WatcherType:      &coordinationv1alpha1.ClusterHandover{},
			WatcherRefGetter: r.dw,
			ClusterScoped:    true,
		}).
		Complete(r)
}

func (r *ClusterHandoverReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("ClusterHandover", req.NamespacedName.String())

	handover := &coordinationv1alpha1.ClusterHandover{}
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
				Namespace: obj.GetNamespace(),
			})
	}

	// report counts
	handover.Status.Stats.Found = int32(len(objListType.Items))
	handover.Status.Stats.Updated = int32(len(newObjs))
	handover.Status.Stats.Available = handover.Status.Stats.Found - int32(unavailable)

	handover.Status.ObservedGeneration = handover.Generation
	if handover.Status.Stats.Found == handover.Status.Stats.Updated && len(handover.Status.Processing) == 0 {
		handover.Status.Phase = coordinationv1alpha1.ClusterHandoverPhaseCompleted
		meta.SetStatusCondition(&handover.Status.Conditions, metav1.Condition{
			Type:               coordinationv1alpha1.ClusterHandoverCompleted,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: handover.Generation,
			Reason:             "Complete",
			Message:            "All found objects have been re-labeled.",
		})
	} else {
		handover.Status.Phase = coordinationv1alpha1.ClusterHandoverPhaseProgressing
		meta.SetStatusCondition(&handover.Status.Conditions, metav1.Condition{
			Type:               coordinationv1alpha1.ClusterHandoverCompleted,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: handover.Generation,
			Reason:             "Incomplete",
			Message:            "Some found objects need to be re-labeled.",
		})
	}

	if err := r.Status().Update(ctx, handover); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating ClusterHandover status: %w", err)
	}
	return ctrl.Result{}, nil
}
