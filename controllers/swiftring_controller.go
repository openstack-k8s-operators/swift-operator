/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/job"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"

	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	swift "github.com/openstack-k8s-operators/swift-operator/pkg/swift"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// SwiftRingReconciler reconciles a SwiftRing object
type SwiftRingReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Log     logr.Logger
	Kclient kubernetes.Interface
}

//+kubebuilder:rbac:groups=swift.openstack.org,resources=swiftrings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=swift.openstack.org,resources=swiftrings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=swift.openstack.org,resources=swiftrings/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=*,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SwiftRing object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *SwiftRingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("swiftring", req.NamespacedName)

	instance := &swiftv1beta1.SwiftRing{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			r.Log.Info("SwiftRing resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.Log.Error(err, "Failed to get SwiftRing")
		return ctrl.Result{}, err
	}

	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
		cl := condition.CreateList(
			condition.UnknownCondition(swiftv1beta1.SwiftRingReadyCondition, condition.InitReason, swiftv1beta1.SwiftRingReadyInitMessage),
		)

		instance.Status.Conditions.Init(&cl)

		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	helper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		r.Log,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !instance.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	ls := swift.GetLabelsRing()

	// Create a Secret populated with content from templates/
	envVars := make(map[string]env.Setter)
	tpl := getRingSecretTemplates(instance, ls)
	err = secret.EnsureSecrets(ctx, helper, instance, tpl, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Swift ring init job - start
	if instance.Status.Hash == nil {
		instance.Status.Hash = map[string]string{}
	}
	ringCreateHash := instance.Status.Hash[swiftv1beta1.RingCreateHash]

	ringCreateJob := job.NewJob(getRingJob(instance, ls), swiftv1beta1.RingCreateHash, false, 5*time.Second, ringCreateHash)
	ctrlResult, err := ringCreateJob.DoJob(ctx, helper)
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			"Ring init job still running"))
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrlResult, nil
	}
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			err.Error()))
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	if ringCreateJob.HasChanged() {
		instance.Status.Hash[swiftv1beta1.RingCreateHash] = ringCreateJob.GetHash()
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		r.Log.V(1).Info(fmt.Sprintf("Job %s hash added - %s", instance.Name, instance.Status.Hash[swiftv1beta1.RingCreateHash]))
	}

	instance.Status.Conditions.MarkTrue(swiftv1beta1.SwiftRingReadyCondition, condition.DeploymentReadyMessage)
	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}
	// Swift ring init job - end

	r.Log.Info(fmt.Sprintf("Reconciled SwiftRing '%s' successfully", instance.Name))
	return ctrl.Result{}, nil
}

func getRingJob(instance *swiftv1beta1.SwiftRing, labels map[string]string) *batchv1.Job {
	securityContext := swift.GetSecurityContext()

	envVars := map[string]env.Setter{}
	envVars["CM_NAME"] = env.SetValue(instance.Spec.RingConfigMap)
	envVars["NAMESPACE"] = env.SetValue(instance.Namespace)
	envVars["STORAGE_POD_PREFIX"] = env.SetValue(instance.Spec.StoragePodPrefix)
	envVars["STORAGE_SVC_NAME"] = env.SetValue(instance.Spec.StorageServiceName)
	envVars["SWIFT_REPLICAS"] = env.SetValue(fmt.Sprint(instance.Spec.RingReplicas))
	envVars["SWIFT_DEVICES"] = env.SetValue(fmt.Sprint(instance.Spec.Devices))
	envVars["OWNER_APIVERSION"] = env.SetValue(instance.APIVersion)
	envVars["OWNER_KIND"] = env.SetValue(instance.Kind)
	envVars["OWNER_UID"] = env.SetValue(string(instance.ObjectMeta.UID))
	envVars["OWNER_NAME"] = env.SetValue(instance.ObjectMeta.Name)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-rebalance",
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      "OnFailure",
					ServiceAccountName: swift.ServiceAccount,
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            instance.Name + "-rebalance",
							Command:         []string{"/usr/local/bin/container-scripts/swift-ring-rebalance.sh"},
							Image:           instance.Spec.ContainerImage,
							SecurityContext: &securityContext,
							VolumeMounts:    getRingVolumeMounts(),
							Env:             env.MergeEnvs([]corev1.EnvVar{}, envVars),
						},
					},
					Volumes: getRingVolumes(instance),
				},
			},
		},
	}
}

func getRingVolumes(instance *swiftv1beta1.SwiftRing) []corev1.Volume {
	var scriptsVolumeDefaultMode int32 = 0755
	return []corev1.Volume{
		{
			Name: "scripts",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &scriptsVolumeDefaultMode,
					SecretName:  instance.Name + "-scripts",
				},
			},
		},
		{
			Name: "swiftconf",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: instance.Spec.SwiftConfSecret,
				},
			},
		},
		{
			Name: "etc-swift",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{Medium: ""},
			},
		},
	}

}

func getRingVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "scripts",
			MountPath: "/usr/local/bin/container-scripts",
			ReadOnly:  true,
		},
		{
			Name:      "swiftconf",
			MountPath: "/var/lib/config-data/swiftconf",
			ReadOnly:  true,
		},
		{
			Name:      "etc-swift",
			MountPath: "/etc/swift",
			ReadOnly:  false,
		},
	}
}

func getRingSecretTemplates(instance *swiftv1beta1.SwiftRing, labels map[string]string) []util.Template {
	return []util.Template{
		{
			Name:         fmt.Sprintf("%s-scripts", instance.Name),
			Namespace:    instance.Namespace,
			Type:         util.TemplateTypeScripts,
			InstanceType: instance.Kind,
			Labels:       labels,
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SwiftRingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&swiftv1beta1.SwiftRing{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Complete(r)
}
