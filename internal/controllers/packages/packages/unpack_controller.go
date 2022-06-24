package packages

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/thetechnick/package-operator/internal/controllers"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type UnpackController struct {
	client              client.Client
	scheme              *runtime.Scheme
	log                 logr.Logger
	newPackage          packageFactory
	newObjectDeployment objectDeploymentFactory
	// where the package content is located
	packagePath string

	loader *packageLoaderBuilder
}

func NewClusterUnpackController(
	log logr.Logger, scheme *runtime.Scheme,
	c client.Client, packagePath string,
) *UnpackController {
	return NewGenericUnpackController(
		log, scheme, c, newClusterPackage, newClusterObjectDeployment, packagePath,
		newClusterPackageLoaderBuilder(log, scheme),
	)
}

func NewUnpackController(
	log logr.Logger, scheme *runtime.Scheme,
	c client.Client, packagePath string,
) *UnpackController {
	return NewGenericUnpackController(
		log, scheme, c, newPackage, newObjectDeployment, packagePath,
		newPackageLoaderBuilder(log, scheme),
	)
}

func NewGenericUnpackController(
	log logr.Logger, scheme *runtime.Scheme,
	c client.Client,
	newPackage packageFactory,
	newObjectDeployment objectDeploymentFactory,
	packagePath string, loader *packageLoaderBuilder,
) *UnpackController {
	uc := &UnpackController{
		client:              c,
		scheme:              scheme,
		log:                 log,
		newPackage:          newPackage,
		newObjectDeployment: newObjectDeployment,
		packagePath:         packagePath,
		loader:              loader,
	}
	return uc
}

func (c *UnpackController) Reconcile(
	ctx context.Context, req ctrl.Request,
) (ctrl.Result, error) {
	log := c.log.WithValues("Package", req.NamespacedName.String())
	ctx = controllers.ContextWithLogger(ctx, log)

	packageObj := c.newPackage(c.scheme)
	if err := c.client.Get(
		ctx, req.NamespacedName, packageObj.ClientObject()); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	deploy, err := c.loader.Load(c.packagePath, packageToContext(packageObj))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("loading package: %w", err)
	}
	deploy.ClientObject().SetName(
		packageObj.ClientObject().GetName())
	deploy.ClientObject().SetNamespace(
		packageObj.ClientObject().GetNamespace())

	if err := controllerutil.SetControllerReference(
		packageObj.ClientObject(),
		deploy.ClientObject(), c.scheme); err != nil {
		return ctrl.Result{}, fmt.Errorf("setting controller reference: %w", err)
	}

	if err := c.reconcileDeployment(ctx, deploy); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (c *UnpackController) reconcileDeployment(ctx context.Context, deploy genericObjectDeployment) error {
	existingDeploy := c.newObjectDeployment(c.scheme)
	err := c.client.Get(ctx, client.ObjectKeyFromObject(deploy.ClientObject()), existingDeploy.ClientObject())
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("getting ObjectDeployment: %w", err)
	}
	if errors.IsNotFound(err) {
		if err := c.client.Create(ctx, deploy.ClientObject()); err != nil {
			return fmt.Errorf("creating ObjectDeployment: %w", err)
		}
	}

	newAnnotations := deploy.ClientObject().GetAnnotations()
	newLabels := deploy.ClientObject().GetLabels()

	// Take existing ObjectMeta
	deploy.SetObjectMeta(existingDeploy.GetObjectMeta())

	// merge annotations and labels
	deployCO := deploy.ClientObject()
	annotations := deployCO.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	for k, v := range newAnnotations {
		annotations[k] = v
	}
	deployCO.SetAnnotations(annotations)

	labels := deployCO.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	for k, v := range newLabels {
		labels[k] = v
	}
	deployCO.SetLabels(labels)

	if err := c.client.Update(ctx, deployCO); err != nil {
		return fmt.Errorf("updating ObjectDeployment: %w", err)
	}
	return nil
}

func packageToContext(pack genericPackage) map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":        pack.ClientObject().GetName(),
			"namespace":   pack.ClientObject().GetNamespace(),
			"annotations": pack.ClientObject().GetAnnotations(),
		},
	}
}
