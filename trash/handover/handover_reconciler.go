package handover

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	"github.com/thetechnick/package-operator/internal/dynamicwatcher"
)

type operand interface {
	coordinationv1alpha1.Handover | coordinationv1alpha1.ClusterHandover
}

type operandPtr[O any] interface {
	client.Object
	*O
}

// Generic reconciler for both Handover and ClusterHandover objects.
type GenericHandoverReconciler[T operandPtr[O], O operand] struct {
	client          client.Client
	log             logr.Logger
	scheme          *runtime.Scheme
	dynamicClient   dynamic.Interface
	discoveryClient *discovery.DiscoveryClient

	dw *dynamicwatcher.DynamicWatcher
}

func NewGenericHandoverReconciler[T operandPtr[O], O operand](
	o O,
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, dynamicClient dynamic.Interface,
	discoveryClient *discovery.DiscoveryClient,
) *GenericHandoverReconciler[T, O] {
	return &GenericHandoverReconciler[T, O]{
		client:          c,
		log:             log,
		scheme:          scheme,
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
	}
}
