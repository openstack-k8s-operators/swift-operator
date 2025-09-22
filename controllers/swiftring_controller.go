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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/job"

	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/pkg/swiftring"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

// SwiftRingReconciler reconciles a SwiftRing object
type SwiftRingReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Kclient kubernetes.Interface
}

// GetLogger returns a logger object with a prefix of "controller.name" and additional controller context fields
func (r *SwiftRingReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("SwiftRing")
}

//+kubebuilder:rbac:groups=swift.openstack.org,resources=swiftrings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=swift.openstack.org,resources=swiftrings/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=swift.openstack.org,resources=swiftrings/finalizers,verbs=update;patch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=*,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SwiftRing object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *SwiftRingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	Log := r.GetLogger(ctx)

	instance := &swiftv1beta1.SwiftRing{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			Log.Info("SwiftRing resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		Log.Error(err, "Failed to get SwiftRing")
		return ctrl.Result{}, err
	}

	helper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		Log,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// initialize status
	//
	// initialize status if Conditions is nil, but do not reset if it
	// already exists
	isNewInstance := instance.Status.Conditions == nil
	if isNewInstance {
		instance.Status.Conditions = condition.Conditions{}
	}

	// Save a copy of the condtions so that we can restore the LastTransitionTime
	// when a condition's state doesn't change.
	savedConditions := instance.Status.Conditions.DeepCopy()

	// Always patch the instance status when exiting this function so we can
	// persist any changes.
	defer func() {
		// Don't update the status, if Reconciler Panics
		if rc := recover(); rc != nil {
			Log.Info(fmt.Sprintf("Panic during reconcile %v\n", rc))
			panic(rc)
		}
		condition.RestoreLastTransitionTimes(
			&instance.Status.Conditions, savedConditions)
		if instance.Status.Conditions.IsUnknown(condition.ReadyCondition) {
			instance.Status.Conditions.Set(
				instance.Status.Conditions.Mirror(condition.ReadyCondition))
		}
		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	cl := condition.CreateList(
		// Mark ReadyCondition as Unknown from the beginning, because the
		// Reconcile function is in progress. If this condition is not marked
		// as True and is still in the "Unknown" state, we `Mirror(` the actual
		// failure/in-progress operation
		condition.UnknownCondition(condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage),
		condition.UnknownCondition(swiftv1beta1.SwiftRingReadyCondition, condition.InitReason, condition.ReadyInitMessage),
	)

	instance.Status.Conditions.Init(&cl)
	// Update the lastObserved generation before evaluating conditions
	instance.Status.ObservedGeneration = instance.Generation

	// If we're not deleting this and the service object doesn't have our finalizer, add it.
	if instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, helper.GetFinalizer()) || isNewInstance {
		return ctrl.Result{}, nil
	}

	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, instance, helper)
}

func (r *SwiftRingReconciler) reconcileNormal(ctx context.Context, instance *swiftv1beta1.SwiftRing, helper *helper.Helper) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)
	Log.Info(fmt.Sprintf("Reconciling Service '%s'", instance.Name))

	serviceLabels := swiftring.Labels()

	// Swift ring init job - start
	if instance.Status.Hash == nil {
		instance.Status.Hash = map[string]string{}
	}

	deviceList, deviceListHash, err := swiftring.DeviceList(ctx, helper, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			swiftv1beta1.SwiftRingReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			swiftv1beta1.SwiftRingReadyErrorMessage,
			err.Error()))
		if apierrors.IsNotFound(err) {
			Log.Info(fmt.Sprintf("%s... requeueing", err.Error()))
			return ctrl.Result{RequeueAfter: time.Duration(5) * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	if instance.Status.Hash[swiftv1beta1.DeviceListHash] != deviceListHash {
		// Create or update the devicelist ConfigMap
		envVars := make(map[string]env.Setter)
		tpl := swiftring.ConfigMapTemplates(instance, serviceLabels, deviceList)
		err = configmap.EnsureConfigMaps(ctx, helper, instance, tpl, &envVars)
		if err != nil {
			return ctrl.Result{}, err
		}

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
			return ctrl.Result{}, nil
		}

		// will only be updated if job does not exist (anymore)
		instance.Status.Hash[swiftv1beta1.RingCreateHash] = ""
		instance.Status.Hash[swiftv1beta1.DeviceListHash] = deviceListHash
	}

	ringCreateJob := job.NewJob(swiftring.GetRingJob(instance, serviceLabels), "rebalance", true, 5*time.Second, instance.Status.Hash[swiftv1beta1.RingCreateHash])
	ctrlResult, err := ringCreateJob.DoJob(ctx, helper)
	if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.ReadyInitMessage))
		return ctrlResult, nil
	}
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.ReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			"%s", err.Error()))
		return ctrl.Result{}, err
	}

	if ringCreateJob.HasChanged() {
		instance.Status.Hash[swiftv1beta1.RingCreateHash] = ringCreateJob.GetHash()
	}

	instance.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)
	instance.Status.Conditions.MarkTrue(swiftv1beta1.SwiftRingReadyCondition, condition.ReadyMessage)
	// Swift ring init job - end

	// We reached the end of the Reconcile, update the Ready condition based on
	// the sub conditions
	if instance.Status.Conditions.AllSubConditionIsTrue() {
		instance.Status.Conditions.MarkTrue(
			condition.ReadyCondition, condition.ReadyMessage)
	}
	Log.Info(fmt.Sprintf("Reconciled SwiftRing '%s' successfully", instance.Name))
	return ctrl.Result{}, nil
}

func (r *SwiftRingReconciler) reconcileDelete(ctx context.Context, instance *swiftv1beta1.SwiftRing, helper *helper.Helper) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)
	Log.Info(fmt.Sprintf("Reconciling Service '%s' delete", instance.Name))

	ringConfigMap, _, err := configmap.GetConfigMapAndHashWithName(ctx, helper, instance.Spec.RingConfigMaps[0], instance.Namespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	if err == nil {
		// This finalizer is directly set when creating the ConfigMap using
		// curl within the Job
		if controllerutil.RemoveFinalizer(ringConfigMap, "openstack.org/swiftring") {
			err = r.Update(ctx, ringConfigMap)
			if err != nil && !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			Log.Info(fmt.Sprintf("Removed finalizer from ConfigMap %s", instance.Spec.RingConfigMaps[0]))
		}
	}

	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	Log.Info(fmt.Sprintf("Reconciled Service '%s' delete successfully", instance.Name))

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SwiftRingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	swiftRingFilter := func(_ context.Context, o client.Object) []reconcile.Request {
		result := []reconcile.Request{}
		swiftRings := &swiftv1beta1.SwiftRingList{}
		listOpts := []client.ListOption{client.InNamespace(o.GetNamespace())}
		err := r.List(context.Background(), swiftRings, listOpts...)
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

	inventoryPredicator, err := predicate.LabelSelectorPredicate(
		metav1.LabelSelector{
			MatchLabels: map[string]string{
				"openstack.org/operator-name": "dataplane",
				"inventory":                   "true",
			},
		},
	)
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&swiftv1beta1.SwiftRing{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ConfigMap{}).
		Watches(&swiftv1beta1.SwiftStorage{}, handler.EnqueueRequestsFromMapFunc(swiftRingFilter)).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(swiftRingFilter),
			builder.WithPredicates(inventoryPredicator)).
		Complete(r)
}
