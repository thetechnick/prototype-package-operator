package handover

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/jsonpath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	"github.com/thetechnick/package-operator/internal/coordination"
	"github.com/thetechnick/package-operator/internal/dynamicwatcher"
	internalprobe "github.com/thetechnick/package-operator/internal/probe"
)

type operand interface {
	coordinationv1alpha1.Handover | coordinationv1alpha1.ClusterHandover
}

type operandPtr[O any] interface {
	client.Object
	*O
}

// Generic reconciler for both Handover and ClusterHandover objects.
type GenericHandoverController[T operandPtr[O], O operand] struct {
	client          client.Client
	log             logr.Logger
	scheme          *runtime.Scheme
	dynamicClient   dynamic.Interface
	discoveryClient *discovery.DiscoveryClient

	dw *dynamicwatcher.DynamicWatcher
}

func NewHandoverController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericHandoverController[*coordinationv1alpha1.Handover, coordinationv1alpha1.Handover] {
	return NewGenericHandoverController(
		coordinationv1alpha1.Handover{},
		c, log, scheme, dynamicClient, discoveryClient,
	)
}

func NewClusterHandoverController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericHandoverController[*coordinationv1alpha1.ClusterHandover, coordinationv1alpha1.ClusterHandover] {
	return NewGenericHandoverController(
		coordinationv1alpha1.ClusterHandover{},
		c, log, scheme, dynamicClient, discoveryClient,
	)
}

func NewGenericHandoverController[T operandPtr[O], O operand](
	o O,
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericHandoverController[T, O] {
	return &GenericHandoverController[T, O]{
		client:          c,
		log:             log,
		scheme:          scheme,
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
	}
}

func (c *GenericHandoverController[T, O]) Reconcile(
	ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := c.log.WithValues("ClusterHandover", req.NamespacedName.String())

	handover := c.newOperand()
	if err := c.client.Get(ctx, req.NamespacedName, handover); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !handover.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, c.handleDeletion(ctx, handover)
	}

	if err := c.ensureCacheFinalizer(ctx, handover); err != nil {
		return ctrl.Result{}, err
	}

	if err := c.ensureWatch(ctx, handover); err != nil {
		return ctrl.Result{}, err
	}

	if res, err := c.reconcile(ctx, handover, log); err != nil {
		return ctrl.Result{}, err
	} else {
		return res, nil
	}
}

func (c *GenericHandoverController[T, O]) SetupWithManager(
	mgr ctrl.Manager) error {
	c.dw = dynamicwatcher.New(
		c.log, c.scheme, c.client.RESTMapper(), c.dynamicClient)
	t := c.newOperand()

	return ctrl.NewControllerManagedBy(mgr).
		For(t).
		Watches(c.dw, &dynamicwatcher.EnqueueWatchingObjects{
			WatcherType:      t,
			WatcherRefGetter: c.dw,
		}).
		Complete(c)
}

func (c *GenericHandoverController[T, O]) newOperand() T {
	var o O
	return T(&o)
}

func (c *GenericHandoverController[T, O]) handleDeletion(
	ctx context.Context, handover T,
) error {
	return coordination.HandleCommonDeletion(ctx, handover, c.client, c.dw)
}

// ensures the cache finalizer is set on the given object
func (c *GenericHandoverController[T, O]) ensureCacheFinalizer(
	ctx context.Context, handover T,
) error {
	return coordination.EnsureCommonFinalizer(ctx, handover, c.client)
}

// ensures the cache is watching the targetAPI
func (c *GenericHandoverController[T, O]) ensureWatch(
	ctx context.Context, handover T,
) error {
	gvk, objType, _ := coordination.UnstructuredFromTargetAPI(
		getTargetAPI(handover))

	if err := c.dw.Watch(handover, objType); err != nil {
		return fmt.Errorf("watching %s: %w", gvk, err)
	}
	return nil
}

func (c *GenericHandoverController[T, O]) reconcile(
	ctx context.Context, handover T, log logr.Logger) (ctrl.Result, error) {
	combinedProbe := parseProbes(handover)

	gvk, objType, objListType := coordination.UnstructuredFromTargetAPI(getTargetAPI(handover))
	strategy := getStrategy(handover)

	// Handle processing objects
	stillProcessing, err := c.handleAllProcessing(ctx, log, strategy,
		objType, combinedProbe, getProcessing(handover))
	if err != nil {
		return ctrl.Result{}, err
	}
	setProcessing(handover, stillProcessing)

	// List all objects
	// select all objects with new or old label value
	requirement, err := labels.NewRequirement(
		strategy.Relabel.LabelKey,
		selection.In,
		[]string{
			strategy.Relabel.ToValue,
			strategy.Relabel.FromValue,
		},
	)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("building selector: %w", err)
	}
	selector := labels.NewSelector().Add(*requirement)

	if err := c.client.List(
		ctx, objListType,
		client.InNamespace(handover.GetNamespace()),
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
		objListType.Items, strategy.Relabel.LabelKey,
		strategy.Relabel.ToValue,
		strategy.Relabel.FromValue,
	)
	newObjs := groups[0]
	oldObjs := groups[1]

	processing := getProcessing(handover)
	for _, obj := range oldObjs {
		if len(processing)+unavailable >= strategy.Relabel.MaxUnavailable {
			break
		}

		// add a new item to the processing queue
		processing = append(
			processing,
			coordinationv1alpha1.HandoverRef{
				UID:       obj.GetUID(),
				Name:      obj.GetName(),
				Namespace: handover.GetNamespace(),
			})
	}
	setProcessing(handover, processing)

	// report counts
	var stats coordinationv1alpha1.HandoverStatusStats
	stats.Found = int32(len(objListType.Items))
	stats.Updated = int32(len(newObjs))
	stats.Available = stats.Found - int32(unavailable)
	setStats(handover, stats)

	if stats.Found == stats.Updated && len(processing) == 0 {
		setStatus(handover, metav1.Condition{
			Type:               coordinationv1alpha1.HandoverCompleted,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: handover.GetGeneration(),
			Reason:             "Complete",
			Message:            "All found objects have been re-labeled.",
		})
	} else {
		setStatus(handover, metav1.Condition{
			Type:               coordinationv1alpha1.HandoverCompleted,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: handover.GetGeneration(),
			Reason:             "Incomplete",
			Message:            "Some found objects need to be re-labeled.",
		})
	}

	if err := c.client.Status().Update(ctx, handover); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating Handover status: %w", err)
	}
	return ctrl.Result{}, nil
}

func (c *GenericHandoverController[T, O]) handleAllProcessing(
	ctx context.Context,
	log logr.Logger,
	handoverStrategy coordinationv1alpha1.HandoverStrategy,
	objType *unstructured.Unstructured,
	probe internalprobe.Interface,
	processing []coordinationv1alpha1.HandoverRef,
) (stillProcessing []coordinationv1alpha1.HandoverRef, err error) {
	for _, handoverRef := range processing {
		finished, err := c.handleSingleProcessing(
			ctx, log, handoverStrategy, objType, probe, handoverRef)
		if err != nil {
			return stillProcessing, err
		}
		if !finished {
			stillProcessing = append(stillProcessing, handoverRef)
		}
	}
	return stillProcessing, nil
}

func (c *GenericHandoverController[T, O]) handleSingleProcessing(
	ctx context.Context,
	log logr.Logger,
	handoverStrategy coordinationv1alpha1.HandoverStrategy,
	objType *unstructured.Unstructured,
	probe internalprobe.Interface,
	handoverRef coordinationv1alpha1.HandoverRef,
) (finished bool, err error) {
	processingObj := objType.DeepCopy()
	err = c.client.Get(ctx, client.ObjectKey{
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
		labels[handoverStrategy.Relabel.LabelKey] != handoverStrategy.Relabel.ToValue {
		labels[handoverStrategy.Relabel.LabelKey] = handoverStrategy.Relabel.ToValue
		processingObj.SetLabels(labels)
		if err := c.client.Update(ctx, processingObj); err != nil {
			return false, fmt.Errorf("updating object in process queue: %w", err)
		}
	}

	jsonPath := jsonpath.New("status-thing!!!").AllowMissingKeys(true)
	// TODO: SOOOO much validation for paths
	if err := jsonPath.Parse("{" + handoverStrategy.Relabel.StatusPath + "}"); err != nil {
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
	if statusValue != handoverStrategy.Relabel.ToValue {
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
