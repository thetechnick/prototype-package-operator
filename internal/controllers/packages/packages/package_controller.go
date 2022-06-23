package packages

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/thetechnick/package-operator/internal/controllers"
	"github.com/thetechnick/package-operator/internal/ownerhandling"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const jobFinalizer = "packages.thetechnick.ninja/job-cleanup"

// Generic reconciler for both Package and ClusterPackage objects.
type GenericPackageController struct {
	newPackage          packageFactory
	newObjectDeployment objectDeploymentFactory
	client              client.Client
	log                 logr.Logger
	scheme              *runtime.Scheme
	jobOwnerStrategy    ownerStrategy
	pkoNamespace        string

	reconciler []reconciler
}

type ownerStrategy interface {
	IsOwner(owner, obj metav1.Object) bool
	ReleaseController(obj metav1.Object)
	SetControllerReference(owner, obj metav1.Object, scheme *runtime.Scheme) error
	EnqueueRequestForOwner(ownerType client.Object, isController bool) handler.EventHandler
}

type packageFactory func(scheme *runtime.Scheme) genericPackage
type objectDeploymentFactory func(scheme *runtime.Scheme) genericObjectDeployment

type reconciler interface {
	Reconcile(ctx context.Context, packageObj genericPackage) (
		ctrl.Result, error)
}

func NewPackageController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, pkoNamespace string,
) *GenericPackageController {
	return NewGenericPackageController(
		newPackage,
		newObjectDeployment,
		c, log, scheme, pkoNamespace,
		// Running all unpack-jobs within the package-operator namespace
		// requires cross-namespace owner handling,
		// which is not available with Native owner handling.
		ownerhandling.Annotation,
	)
}

func NewClusterPackageController(
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, pkoNamespace string,
) *GenericPackageController {
	return NewGenericPackageController(
		newClusterPackage,
		newClusterObjectDeployment,
		c, log, scheme, pkoNamespace,
		ownerhandling.Native,
	)
}

func NewGenericPackageController(
	newPackage packageFactory,
	newObjectDeployment objectDeploymentFactory,
	c client.Client, log logr.Logger,
	scheme *runtime.Scheme, pkoNamespace string,
	jobOwnerStrategy ownerStrategy,
) *GenericPackageController {
	controller := &GenericPackageController{
		client:              c,
		log:                 log,
		scheme:              scheme,
		newPackage:          newPackage,
		newObjectDeployment: newObjectDeployment,
		jobOwnerStrategy:    jobOwnerStrategy,
		pkoNamespace:        pkoNamespace,
	}

	controller.reconciler = []reconciler{
		newHashReconciler(),
		newUnpackReconciler(c, scheme, pkoNamespace, jobOwnerStrategy),
		newObjectDeploymentReconciler(c, scheme, newObjectDeployment),
	}

	return controller
}

func (c *GenericPackageController) SetupWithManager(mgr ctrl.Manager) error {
	t := c.newPackage(c.scheme).ClientObject()

	return ctrl.NewControllerManagedBy(mgr).
		For(t).
		Owns(c.newObjectDeployment(c.scheme).ClientObject()).
		Watches(
			&source.Kind{
				Type: &batchv1.Job{},
			},
			c.jobOwnerStrategy.EnqueueRequestForOwner(t, false),
		).
		Complete(c)
}

func (c *GenericPackageController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("Package", req.NamespacedName.String())
	ctx = controllers.ContextWithLogger(ctx, log)

	packageObj := c.newPackage(c.scheme)
	if err := c.client.Get(
		ctx, req.NamespacedName, packageObj.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !packageObj.ClientObject().GetDeletionTimestamp().IsZero() {
		// Package was deleted.
		return ctrl.Result{}, c.handleDeletion(ctx, packageObj)
	}

	if err := c.ensureCacheFinalizer(ctx, packageObj); err != nil {
		return ctrl.Result{}, err
	}

	var (
		res ctrl.Result
		err error
	)
	for _, r := range c.reconciler {
		res, err = r.Reconcile(ctx, packageObj)
		if err != nil || !res.IsZero() {
			break
		}
	}
	if err != nil {
		return res, err
	}

	packageObj.UpdatePhase()
	return res, c.client.Status().Update(ctx, packageObj.ClientObject())
}

// ensures the cache finalizer is set on the given object
func (c *GenericPackageController) ensureCacheFinalizer(
	ctx context.Context, pack genericPackage,
) error {
	return controllers.EnsureCommonFinalizer(ctx, pack.ClientObject(), c.client, jobFinalizer)
}

func (c *GenericPackageController) handleDeletion(
	ctx context.Context, pack genericPackage,
) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      unpackJobName(pack),
			Namespace: c.pkoNamespace,
		},
	}
	err := c.client.Delete(ctx, job)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("cleaning up unpack Job: %w", err)
	}

	obj := pack.ClientObject()
	if controllerutil.ContainsFinalizer(obj, jobFinalizer) {
		controllerutil.RemoveFinalizer(obj, jobFinalizer)

		if err := c.client.Update(ctx, obj); err != nil {
			return fmt.Errorf("removing finalizer: %w", err)
		}
	}
	return nil
}
