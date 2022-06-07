package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	"github.com/thetechnick/package-operator/internal/dynamicwatcher"
)

type ClusterAdoptionReconciler struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	DynamicClient   dynamic.Interface
	DiscoveryClient *discovery.DiscoveryClient

	dw *dynamicwatcher.DynamicWatcher
}

func (r *ClusterAdoptionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.dw = dynamicwatcher.New(
		r.Log, r.Scheme, r.RESTMapper(), r.DynamicClient)

	return ctrl.NewControllerManagedBy(mgr).
		For(&coordinationv1alpha1.ClusterAdoption{}).
		Watches(r.dw, &dynamicwatcher.EnqueueWatchingObjects{
			WatcherType:      &coordinationv1alpha1.ClusterAdoption{},
			WatcherRefGetter: r.dw,
			ClusterScoped:    true,
		}).
		Complete(r)
}

func (r *ClusterAdoptionReconciler) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	adoption := &coordinationv1alpha1.ClusterAdoption{}
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
		Type:               coordinationv1alpha1.ClusterAdoptionActive,
		Status:             metav1.ConditionTrue,
		Reason:             "Setup",
		Message:            "Controller is setup and adding labels.",
		ObservedGeneration: adoption.Generation,
	})
	adoption.Status.ObservedGeneration = adoption.Generation
	adoption.Status.Phase = coordinationv1alpha1.ClusterAdoptionPhaseActive

	return ctrl.Result{}, r.Status().Update(ctx, adoption)
}
