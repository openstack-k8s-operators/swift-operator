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
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/job"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"

	dataplanev1 "github.com/openstack-k8s-operators/dataplane-operator/api/v1beta1"
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/pkg/swiftring"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
			condition.UnknownCondition(condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage),
			condition.UnknownCondition(swiftv1beta1.SwiftRingReadyCondition, condition.InitReason, condition.ReadyInitMessage),
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

	serviceLabels := swiftring.Labels()

	// Create a Secret populated with content from templates/
	envVars := make(map[string]env.Setter)
	tpl := swiftring.SecretTemplates(instance, serviceLabels)
	err = secret.EnsureSecrets(ctx, helper, instance, tpl, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Swift ring init job - start
	if instance.Status.Hash == nil {
		instance.Status.Hash = map[string]string{}
	}

	deviceList, deviceListHash, err := swiftring.DeviceList(ctx, helper, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create the devicelist ConfigMap
	envVars = make(map[string]env.Setter)
	tpl = swiftring.DeviceConfigMapTemplates(instance, deviceList)
	err = configmap.EnsureConfigMaps(ctx, helper, instance, tpl, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	if instance.Status.Hash[swiftv1beta1.DeviceListHash] != deviceListHash {
		// Delete a possibly still existing job that finished to re-run the job
		j, err := job.GetJobWithName(ctx, helper, instance.Name+"-rebalance", instance.Namespace)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
		} else {
			if j.Status.Active == 0 {
				err = job.DeleteJob(ctx, helper, instance.Name+"-rebalance", instance.Namespace)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
		}

		instance.Status.Hash[swiftv1beta1.RingCreateHash] = ""
		instance.Status.Hash[swiftv1beta1.DeviceListHash] = deviceListHash
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	ringCreateJob := job.NewJob(swiftring.GetRingJob(instance, serviceLabels), "rebalance", false, 5*time.Second, instance.Status.Hash[swiftv1beta1.RingCreateHash])
	ctrlResult, err := ringCreateJob.DoJob(ctx, helper)
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.ReadyInitMessage))
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
		instance.Status.Hash[swiftv1beta1.DeviceListHash] = deviceListHash
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Make sure the ring file ConfigMap does exist and did not get deleted
	_, ctrlResult, err = configmap.GetConfigMap(ctx, helper, instance, swiftv1beta1.RingConfigMapName, 5*time.Second)
	if err != nil {
		return ctrlResult, err
	}
	// err is nil if not found, thus checking ctrl.Result
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Hash[swiftv1beta1.DeviceListHash] = ""
		instance.Status.Hash[swiftv1beta1.RingCreateHash] = ""
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrlResult, nil
	}

	instance.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)
	instance.Status.Conditions.MarkTrue(swiftv1beta1.SwiftRingReadyCondition, condition.ReadyMessage)
	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}
	// Swift ring init job - end

	r.Log.Info(fmt.Sprintf("Reconciled SwiftRing '%s' successfully", instance.Name))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SwiftRingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	getSwiftRings := func(o client.Object, result []reconcile.Request) []reconcile.Request {
		// There should be only one SwiftRing instance within
		// the Namespace - that needs to be reconciled
		swiftRings := &swiftv1beta1.SwiftRingList{}
		listOpts := []client.ListOption{client.InNamespace(o.GetNamespace())}
		err := r.Client.List(context.Background(), swiftRings, listOpts...)
		if err != nil {
			return nil
		}
		for _, cr := range swiftRings.Items {
			name := client.ObjectKey{
				Namespace: o.GetNamespace(),
				Name:      cr.Name,
			}
			result = append(result, reconcile.Request{NamespacedName: name})
		}
		return result
	}

	deviceConfigMapFilter := func(ctx context.Context, o client.Object) []reconcile.Request {
		result := []reconcile.Request{}
		if (o.GetName() == swiftv1beta1.DeviceConfigMapName) || (o.GetName() == swiftv1beta1.RingConfigMapName) {
			return getSwiftRings(o, result)
		}
		return result
	}

	swiftRingFilter := func(ctx context.Context, o client.Object) []reconcile.Request {
		result := []reconcile.Request{}
		return getSwiftRings(o, result)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&swiftv1beta1.SwiftRing{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&rbacv1.ClusterRole{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(deviceConfigMapFilter)).
		Watches(&swiftv1beta1.SwiftStorage{}, handler.EnqueueRequestsFromMapFunc(swiftRingFilter)).
		Watches(&dataplanev1.OpenStackDataPlaneNodeSet{}, handler.EnqueueRequestsFromMapFunc(swiftRingFilter)).
		Complete(r)
}
