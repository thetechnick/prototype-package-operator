package packages

import (
	"context"
	"fmt"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type unpackReconciler struct {
	client           client.Client
	scheme           *runtime.Scheme
	pkoNamespace     string
	jobOwnerStrategy ownerStrategy
}

func newUnpackReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	pkoNamespace string,
	jobOwnerStrategy ownerStrategy,
) *unpackReconciler {
	return &unpackReconciler{
		client:           client,
		scheme:           scheme,
		pkoNamespace:     pkoNamespace,
		jobOwnerStrategy: jobOwnerStrategy,
	}
}

func (c *unpackReconciler) Reconcile(
	ctx context.Context, packageObj genericPackage,
) (ctrl.Result, error) {
	return ctrl.Result{}, c.ensureUnpack(ctx, packageObj)
}

func (c *unpackReconciler) ensureUnpack(
	ctx context.Context, packageObj genericPackage,
) error {
	job, err := c.ensureUnpackJob(ctx, packageObj)
	if err != nil {
		return fmt.Errorf("ensure unpack job: %w", err)
	}

	var jobCompleted bool
	for _, cond := range job.Status.Conditions {
		if cond.Type == batchv1.JobComplete &&
			cond.Status == corev1.ConditionTrue {
			jobCompleted = true
			meta.SetStatusCondition(
				packageObj.GetConditions(), metav1.Condition{
					Type:               packagesv1alpha1.PackageUnpacked,
					Status:             metav1.ConditionTrue,
					Reason:             "UnpackSuccess",
					Message:            "Unpack job succeeded",
					ObservedGeneration: packageObj.ClientObject().GetGeneration(),
				})
			continue
		}

		if cond.Type == batchv1.JobFailed &&
			cond.Status == corev1.ConditionTrue {
			jobCompleted = true
			meta.SetStatusCondition(
				packageObj.GetConditions(), metav1.Condition{
					Type:               packagesv1alpha1.PackageUnpacked,
					Status:             metav1.ConditionFalse,
					Reason:             "UnpackFailure",
					Message:            "Unpack job failed",
					ObservedGeneration: packageObj.ClientObject().GetGeneration(),
				})
			if err := c.client.Delete(ctx, job); err != nil {
				return fmt.Errorf("deleting failed job: %w", err)
			}
		}
	}

	if !jobCompleted {
		meta.SetStatusCondition(
			packageObj.GetConditions(), metav1.Condition{
				Type:               packagesv1alpha1.PackageUnpacked,
				Status:             metav1.ConditionFalse,
				Reason:             "Unpacking",
				Message:            "Unpack job in progress",
				ObservedGeneration: packageObj.ClientObject().GetGeneration(),
			})
	}
	return nil
}

const packageSourceHashAnnotation = "packages.thetechnick.ninja/package-source-hash"

func (c *unpackReconciler) ensureUnpackJob(
	ctx context.Context, packageObj genericPackage,
) (*batchv1.Job, error) {
	desiredJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      unpackJobName(packageObj),
			Namespace: c.pkoNamespace,
			Annotations: map[string]string{
				packageSourceHashAnnotation: packageObj.GetStatusSourceHash(),
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: pointer.Int32(300),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: "package-loader",
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					InitContainers: []corev1.Container{
						{ // copy static loader binary into a volume
							// Image: "quay.io/nschiede/package-loader:" + version.Version,
							Image: "quay.io/nschiede/package-loader:bed90d3",
							Name:  "prepare-loader",
							Command: []string{
								"cp", "/package-loader", "/loader-bin/package-loader",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "loader-bin",
									MountPath: "/loader-bin",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{ // run loader binary against content from the package image
							Image: packageObj.GetImage(),
							Name:  "load",
							Command: []string{
								"/.loader-bin/package-loader",
								"-package-path=/package",
								"-package-name", packageObj.ClientObject().GetName(),
								"-package-namespace", packageObj.ClientObject().GetNamespace(),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name: "loader-bin",
									// loader skips dot-folders and files
									MountPath: "/.loader-bin",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "loader-bin",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
	if err := c.jobOwnerStrategy.SetControllerReference(
		packageObj.ClientObject(), desiredJob, c.scheme); err != nil {
		return nil, fmt.Errorf("set controller reference: %w", err)
	}

	existingJob := &batchv1.Job{}
	if err := c.client.Get(ctx, client.ObjectKeyFromObject(desiredJob), existingJob); err != nil && errors.IsNotFound(err) {
		if err := c.client.Create(ctx, desiredJob); err != nil {
			return nil, fmt.Errorf("creating Job: %w", err)
		}
		return desiredJob, nil
	} else if err != nil {
		return nil, fmt.Errorf("getting Job: %w", err)
	}

	if existingJob.Annotations == nil ||
		existingJob.Annotations[packageSourceHashAnnotation] !=
			desiredJob.Annotations[packageSourceHashAnnotation] {
		// re-create job
		if err := c.client.Delete(ctx, existingJob); err != nil {
			return nil, fmt.Errorf("deleting outdated Job: %w", err)
		}
		if err := c.client.Create(ctx, desiredJob); err != nil {
			return nil, fmt.Errorf("creating Job: %w", err)
		}
		return desiredJob, nil
	}

	return existingJob, nil
}

func unpackJobName(packageObj genericPackage) string {
	ns := packageObj.ClientObject().GetNamespace()
	if len(ns) > 0 {
		return packageObj.ClientObject().GetName() + "-" + ns + "-unpack"
	}
	return packageObj.ClientObject().GetName() + "-unpack"
}
