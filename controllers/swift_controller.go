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

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	common_rbac "github.com/openstack-k8s-operators/lib-common/modules/common/rbac"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	swiftv1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	swift "github.com/openstack-k8s-operators/swift-operator/pkg/swift"
	"k8s.io/client-go/kubernetes"
)

// SwiftReconciler reconciles a Swift object
type SwiftReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Kclient kubernetes.Interface
}

// GetLogger returns a logger object with a prefix of "controller.name" and additional controller context fields
func (r *SwiftReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("Swift")
}

//+kubebuilder:rbac:groups=swift.openstack.org,resources=swifts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=swift.openstack.org,resources=swifts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=swift.openstack.org,resources=swifts/finalizers,verbs=update;patch

// service account, role, rolebinding
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch
// service account permissions that are needed to grant permission to the above
// +kubebuilder:rbac:groups="security.openshift.io",resourceNames=nonroot-v2,resources=securitycontextconstraints,verbs=use

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Swift object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *SwiftReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	Log := r.GetLogger(ctx)

	instance := &swiftv1.Swift{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			Log.Info("Swift resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		Log.Error(err, "Failed to get Swift")
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

	// initialize status if Conditions is nil, but do not reset if it already
	// exists
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

	// initialize status

	// initialize conditions used later as Status=Unknown
	cl := condition.CreateList(
		// Mark ReadyCondition as Unknown from the beginning, because the
		// Reconcile function is in progress. If this condition is not marked
		// as True and is still in the "Unknown" state, we `Mirror(` the actual
		// failure/in-progress operation
		condition.UnknownCondition(condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage),
		condition.UnknownCondition(condition.ServiceConfigReadyCondition, condition.InitReason, condition.ServiceConfigReadyInitMessage),
		condition.UnknownCondition(swiftv1.SwiftProxyReadyCondition, condition.InitReason, swiftv1.SwiftProxyReadyInitMessage),
		condition.UnknownCondition(swiftv1.SwiftRingReadyCondition, condition.InitReason, swiftv1.SwiftRingReadyInitMessage),
		condition.UnknownCondition(swiftv1.SwiftStorageReadyCondition, condition.InitReason, swiftv1.SwiftStorageReadyInitMessage),
		// service account, role, rolebinding conditions
		condition.UnknownCondition(condition.ServiceAccountReadyCondition, condition.InitReason, condition.ServiceAccountReadyInitMessage),
		condition.UnknownCondition(condition.RoleReadyCondition, condition.InitReason, condition.RoleReadyInitMessage),
		condition.UnknownCondition(condition.RoleBindingReadyCondition, condition.InitReason, condition.RoleBindingReadyInitMessage),
		condition.UnknownCondition(condition.MemcachedReadyCondition, condition.InitReason, condition.MemcachedReadyInitMessage),
	)

	instance.Status.Conditions.Init(&cl)
	// Update the lastObserved generation before evaluating conditions
	instance.Status.ObservedGeneration = instance.Generation

	// If we're not deleting this and the service object doesn't have our finalizer, add it.
	if instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, helper.GetFinalizer()) || isNewInstance {
		return ctrl.Result{}, nil
	}

	// Handle service delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, instance, helper)
}

func (r *SwiftReconciler) reconcileNormal(ctx context.Context, instance *swiftv1.Swift, helper *helper.Helper) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)
	Log.Info(fmt.Sprintf("Reconciling Service '%s'", instance.Name))

	// Service account, role, binding
	rbacRules := []rbacv1.PolicyRule{
		{
			APIGroups:     []string{"security.openshift.io"},
			ResourceNames: []string{"nonroot-v2"},
			Resources:     []string{"securitycontextconstraints"},
			Verbs:         []string{"use"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"create"},
		},
		{
			APIGroups:     []string{""},
			Resources:     []string{"configmaps"},
			ResourceNames: []string{"swift-ring-files"},
			Verbs:         []string{"get", "update", "delete"},
		},
	}
	rbacResult, err := common_rbac.ReconcileRbac(ctx, helper, instance, rbacRules)
	if err != nil {
		return rbacResult, err
	} else if (rbacResult != ctrl.Result{}) {
		return rbacResult, nil
	}

	serviceLabels := swift.Labels()

	// Create a Secret populated with content from templates/, but only if
	// it does not exist yet. Human operators might create a Secret in
	// advance if migrating from an existing deployment
	_, _, err = secret.GetSecret(ctx, helper, swift.SwiftConfSecretName, instance.Namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			envVars := make(map[string]env.Setter)
			tpl := swift.SecretTemplates(instance, serviceLabels)
			err = secret.EnsureSecrets(ctx, helper, instance, tpl, &envVars)
			if err != nil {
				instance.Status.Conditions.Set(condition.FalseCondition(
					condition.ServiceConfigReadyCondition,
					condition.ErrorReason,
					condition.SeverityWarning,
					condition.ServiceConfigReadyErrorMessage,
					err.Error()))
				return ctrl.Result{}, err
			}
		} else {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.ServiceConfigReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.ServiceConfigReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}
	}

	instance.Status.Conditions.MarkTrue(condition.ServiceConfigReadyCondition, condition.ServiceConfigReadyMessage)

	//
	// Check for required memcached used for caching
	//
	memcached, err := memcachedv1.GetMemcachedByName(ctx, helper, instance.Spec.MemcachedInstance, instance.Namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.MemcachedReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.MemcachedReadyWaitingMessage))
			Log.Info(fmt.Sprintf("%s... requeueing", condition.MemcachedReadyWaitingMessage))
			return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil

		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.MemcachedReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.MemcachedReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	if !memcached.IsReady() {
		instance.Status.Conditions.Set(condition.FalseCondition(
			condition.MemcachedReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.MemcachedReadyWaitingMessage))
		Log.Info(fmt.Sprintf("%s... requeueing", condition.MemcachedReadyWaitingMessage))
		return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
	}
	// Mark the Memcached Service as Ready if we get to this point with no errors
	instance.Status.Conditions.MarkTrue(
		condition.MemcachedReadyCondition, condition.MemcachedReadyMessage)
	// run check memcached - end

	// create or update Swift storage
	swiftStorage, op, err := r.storageCreateOrUpdate(ctx, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			swiftv1.SwiftStorageReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			swiftv1.SwiftStorageReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	// make sure the controller is watching the last generation of the subCR
	stg, err := r.checkSwiftStorageGeneration(ctx, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			swiftv1.SwiftStorageReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			swiftv1.SwiftStorageReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if !stg {
		instance.Status.Conditions.Set(condition.UnknownCondition(
			swiftv1.SwiftStorageReadyCondition,
			condition.InitReason,
			swiftv1.SwiftStorageReadyInitMessage,
		))
	} else {
		// Mirror SwiftStorage's condition status
		c := swiftStorage.Status.Conditions.Mirror(swiftv1.SwiftStorageReadyCondition)
		if c != nil {
			instance.Status.Conditions.Set(c)
		}
	}
	if op != controllerutil.OperationResultNone && stg {
		Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	if instance.Spec.SwiftRing.Enabled {
		// create or update Swift rings
		swiftRing, op, err := r.ringCreateOrUpdate(ctx, instance)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				swiftv1.SwiftRingReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				swiftv1.SwiftRingReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}

		// make sure the controller is watching the last generation of the subCR
		ring, err := r.checkSwiftRingGeneration(ctx, instance)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				swiftv1.SwiftRingReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				swiftv1.SwiftRingReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}
		if !ring {
			instance.Status.Conditions.Set(condition.UnknownCondition(
				swiftv1.SwiftRingReadyCondition,
				condition.InitReason,
				swiftv1.SwiftRingReadyInitMessage,
			))
		} else {
			// Mirror SwiftRing's condition status
			c := swiftRing.Status.Conditions.Mirror(swiftv1.SwiftRingReadyCondition)
			if c != nil {
				instance.Status.Conditions.Set(c)
			}
		}

		if op != controllerutil.OperationResultNone && ring {
			Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
		}
	} else {
		instance.Status.Conditions.Remove(swiftv1.SwiftRingReadyCondition)
	}

	// create or update Swift proxy
	swiftProxy, op, err := r.proxyCreateOrUpdate(ctx, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			swiftv1.SwiftProxyReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			swiftv1.SwiftProxyReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	sst, err := r.checkSwiftProxyGeneration(ctx, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			swiftv1.SwiftProxyReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			swiftv1.SwiftProxyReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if !sst {
		instance.Status.Conditions.Set(condition.UnknownCondition(
			swiftv1.SwiftProxyReadyCondition,
			condition.InitReason,
			swiftv1.SwiftProxyReadyInitMessage,
		))
	} else {
		// Mirror SwiftProxy's condition status
		c := swiftProxy.Status.Conditions.Mirror(swiftv1.SwiftProxyReadyCondition)
		if c != nil {
			instance.Status.Conditions.Set(c)
		}
	}
	if op != controllerutil.OperationResultNone && sst {
		Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// We reached the end of the Reconcile, update the Ready condition based on
	// the sub conditions
	if instance.Status.Conditions.AllSubConditionIsTrue() {
		instance.Status.Conditions.MarkTrue(
			condition.ReadyCondition, condition.ReadyMessage)
	}
	Log.Info(fmt.Sprintf("Reconciled Service '%s' successfully", instance.Name))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SwiftReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&swiftv1.Swift{}).
		Owns(&swiftv1.SwiftRing{}).
		Owns(&swiftv1.SwiftStorage{}).
		Owns(&swiftv1.SwiftProxy{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}

func (r *SwiftReconciler) reconcileDelete(ctx context.Context, instance *swiftv1.Swift, helper *helper.Helper) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)
	Log.Info(fmt.Sprintf("Reconciling Service '%s' delete", instance.Name))

	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	Log.Info(fmt.Sprintf("Reconciled Service '%s' delete successfully", instance.Name))

	return ctrl.Result{}, nil
}

func (r *SwiftReconciler) ringCreateOrUpdate(ctx context.Context, instance *swiftv1.Swift) (*swiftv1.SwiftRing, controllerutil.OperationResult, error) {

	swiftRingSpec := swiftv1.SwiftRingSpec{
		ContainerImage: instance.Spec.SwiftRing.ContainerImage,
		SwiftRingSpecCore: swiftv1.SwiftRingSpecCore{
			RingReplicas:   instance.Spec.SwiftRing.RingReplicas,
			PartPower:      instance.Spec.SwiftRing.PartPower,
			MinPartHours:   instance.Spec.SwiftRing.MinPartHours,
			TLS:            instance.Spec.SwiftProxy.TLS.Ca,
			NodeSelector:   instance.Spec.SwiftRing.NodeSelector,
			RingConfigMaps: instance.Spec.RingConfigMaps,
		},
	}

	if swiftRingSpec.NodeSelector == nil {
		swiftRingSpec.NodeSelector = instance.Spec.NodeSelector
	}

	deployment := &swiftv1.SwiftRing{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-ring", instance.Name),
			Namespace: instance.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		deployment.Spec = swiftRingSpec
		err := controllerutil.SetControllerReference(instance, deployment, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})

	return deployment, op, err
}

func (r *SwiftReconciler) storageCreateOrUpdate(ctx context.Context, instance *swiftv1.Swift) (*swiftv1.SwiftStorage, controllerutil.OperationResult, error) {

	swiftStorageSpec := swiftv1.SwiftStorageSpec{
		ContainerImageAccount:   instance.Spec.SwiftStorage.ContainerImageAccount,
		ContainerImageContainer: instance.Spec.SwiftStorage.ContainerImageContainer,
		ContainerImageObject:    instance.Spec.SwiftStorage.ContainerImageObject,
		ContainerImageProxy:     instance.Spec.SwiftStorage.ContainerImageProxy,
		SwiftStorageSpecCore: swiftv1.SwiftStorageSpecCore{
			Replicas:                instance.Spec.SwiftStorage.Replicas,
			StorageClass:            instance.Spec.SwiftStorage.StorageClass,
			StorageRequest:          instance.Spec.SwiftStorage.StorageRequest,
			NetworkAttachments:      instance.Spec.SwiftStorage.NetworkAttachments,
			MemcachedInstance:       instance.Spec.MemcachedInstance,
			ContainerSharderEnabled: instance.Spec.SwiftStorage.ContainerSharderEnabled,
			DefaultConfigOverwrite:  instance.Spec.SwiftStorage.DefaultConfigOverwrite,
			NodeSelector:            instance.Spec.SwiftStorage.NodeSelector,
			TopologyRef:             instance.Spec.SwiftProxy.TopologyRef,
			RingConfigMaps:          instance.Spec.RingConfigMaps,
		},
	}

	if swiftStorageSpec.NodeSelector == nil {
		swiftStorageSpec.NodeSelector = instance.Spec.NodeSelector
	}

	// If topology is not present in the underlying SwiftStorage Spec
	// inherit from the top-level CR
	if swiftStorageSpec.TopologyRef == nil {
		swiftStorageSpec.TopologyRef = instance.Spec.TopologyRef
	}

	deployment := &swiftv1.SwiftStorage{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-storage", instance.Name),
			Namespace: instance.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		deployment.Spec = swiftStorageSpec
		err := controllerutil.SetControllerReference(instance, deployment, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})

	return deployment, op, err
}

func (r *SwiftReconciler) proxyCreateOrUpdate(ctx context.Context, instance *swiftv1.Swift) (*swiftv1.SwiftProxy, controllerutil.OperationResult, error) {

	swiftProxySpec := swiftv1.SwiftProxySpec{
		ContainerImageProxy: instance.Spec.SwiftProxy.ContainerImageProxy,
		SwiftProxySpecCore: swiftv1.SwiftProxySpecCore{
			Replicas:               instance.Spec.SwiftProxy.Replicas,
			Secret:                 instance.Spec.SwiftProxy.Secret,
			ServiceUser:            instance.Spec.SwiftProxy.ServiceUser,
			PasswordSelectors:      instance.Spec.SwiftProxy.PasswordSelectors,
			Override:               instance.Spec.SwiftProxy.Override,
			NetworkAttachments:     instance.Spec.SwiftProxy.NetworkAttachments,
			MemcachedInstance:      instance.Spec.MemcachedInstance,
			TLS:                    instance.Spec.SwiftProxy.TLS,
			DefaultConfigOverwrite: instance.Spec.SwiftProxy.DefaultConfigOverwrite,
			EncryptionEnabled:      instance.Spec.SwiftProxy.EncryptionEnabled,
			RabbitMqClusterName:    instance.Spec.SwiftProxy.RabbitMqClusterName,
			CeilometerEnabled:      instance.Spec.SwiftProxy.CeilometerEnabled,
			NodeSelector:           instance.Spec.SwiftProxy.NodeSelector,
			TopologyRef:            instance.Spec.SwiftProxy.TopologyRef,
			RingConfigMaps:         instance.Spec.RingConfigMaps,
		},
	}

	if swiftProxySpec.NodeSelector == nil {
		swiftProxySpec.NodeSelector = instance.Spec.NodeSelector
	}

	// If topology is not present in the underlying SwiftProxy Spec,
	// inherit from the top-level CR
	if swiftProxySpec.TopologyRef == nil {
		swiftProxySpec.TopologyRef = instance.Spec.TopologyRef
	}

	deployment := &swiftv1.SwiftProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-proxy", instance.Name),
			Namespace: instance.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {
		deployment.Spec = swiftProxySpec
		err := controllerutil.SetControllerReference(instance, deployment, r.Scheme)
		if err != nil {
			return err
		}

		return nil
	})

	return deployment, op, err
}

// checkSwiftProxyGeneration -
func (r *SwiftReconciler) checkSwiftProxyGeneration(
	ctx context.Context,
	instance *swiftv1.Swift,
) (bool, error) {
	Log := r.GetLogger(ctx)
	proxy := &swiftv1.SwiftProxyList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	if err := r.List(context.Background(), proxy, listOpts...); err != nil {
		Log.Error(err, "Unable to retrieve SwiftProxy %w")
		return false, err
	}
	for _, item := range proxy.Items {
		if item.Generation != item.Status.ObservedGeneration {
			return false, nil
		}
	}
	return true, nil
}

// checkSwiftStorageGeneration -
func (r *SwiftReconciler) checkSwiftStorageGeneration(
	ctx context.Context,
	instance *swiftv1.Swift,
) (bool, error) {
	Log := r.GetLogger(ctx)
	sst := &swiftv1.SwiftStorageList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	if err := r.List(context.Background(), sst, listOpts...); err != nil {
		Log.Error(err, "Unable to retrieve SwiftStorage %w")
		return false, err
	}
	for _, item := range sst.Items {
		if item.Generation != item.Status.ObservedGeneration {
			return false, nil
		}
	}
	return true, nil
}

// checkSwiftRingGeneration -
func (r *SwiftReconciler) checkSwiftRingGeneration(
	ctx context.Context,
	instance *swiftv1.Swift,
) (bool, error) {
	Log := r.GetLogger(ctx)
	rings := &swiftv1.SwiftRingList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	if err := r.List(context.Background(), rings, listOpts...); err != nil {
		Log.Error(err, "Unable to retrieve SwiftRing %w")
		return false, err
	}
	for _, item := range rings.Items {
		if item.Generation != item.Status.ObservedGeneration {
			return false, nil
		}
	}
	return true, nil
}
