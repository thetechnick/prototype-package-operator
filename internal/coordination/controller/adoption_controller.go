package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	"github.com/thetechnick/package-operator/internal/dynamicwatcher"
)

type AdoptionReconciler struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	DynamicClient   dynamic.Interface
	DiscoveryClient *discovery.DiscoveryClient

	dw *dynamicwatcher.DynamicWatcher
}

func (r *AdoptionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.dw = dynamicwatcher.New(
		r.Log, r.Scheme, r.RESTMapper(), r.DynamicClient)

	return ctrl.NewControllerManagedBy(mgr).
		For(&coordinationv1alpha1.Adoption{}).
		Watches(r.dw, &dynamicwatcher.EnqueueWatchingObjects{
			WatcherType:      &coordinationv1alpha1.Adoption{},
			WatcherRefGetter: r.dw,
		}).
		Complete(r)
}

func (r *AdoptionReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	adoption := &coordinationv1alpha1.Adoption{}
	if err := r.Get(ctx, req.NamespacedName, adoption); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !adoption.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, handleDeletion(ctx, r.Client, r.dw, adoption)
	}

	// Add finalizers
	if err := ensureCacheFinalizer(ctx, r.Client, adoption); err != nil {
		return ctrl.Result{}, err
	}

	// Ensure watch
	gvk, objType, objListType := unstructuredFromTargetAPI(adoption.Spec.TargetAPI)
	if err := r.dw.Watch(adoption, objType); err != nil {
		return ctrl.Result{}, fmt.Errorf("watching %s: %w", gvk, err)
	}

	// Relabel stuff
	if err := relabel(ctx, r.Client, adoption, adoption.Spec.Strategy.Static.Labels, objListType, gvk); err != nil {
		return ctrl.Result{}, err
	}

	meta.SetStatusCondition(&adoption.Status.Conditions, metav1.Condition{
		Type:    coordinationv1alpha1.AdoptionActive,
		Status:  metav1.ConditionTrue,
		Reason:  "Setup",
		Message: "Controller is setup and adding labels.",
	})

	return ctrl.Result{}, nil
}

func relabel(
	ctx context.Context, c client.Client,
	adoption client.Object, specLabels map[string]string,
	objListType *unstructured.UnstructuredList,
	gvk schema.GroupVersionKind,
) error {
	// Build selector
	selector := labels.NewSelector()
	for k := range specLabels {
		requirement, err := labels.NewRequirement(
			k, selection.DoesNotExist, nil)
		if err != nil {
			return fmt.Errorf("building selector: %w", err)
		}
		selector.Add(*requirement)
	}

	// List all the things!
	if err := c.List(
		ctx, objListType,
		client.InNamespace(adoption.GetNamespace()), // can also set this for ClusterAdoption without issue.
		client.MatchingLabelsSelector{
			Selector: selector,
		},
	); err != nil {
		return fmt.Errorf("listing %s: %w", gvk, err)
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

		if err := c.Update(ctx, &obj); err != nil {
			return fmt.Errorf("setting labels: %w", err)
		}
	}

	return nil
}
