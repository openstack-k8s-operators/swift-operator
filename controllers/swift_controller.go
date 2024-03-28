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
	Log     logr.Logger
	Kclient kubernetes.Interface
}

//+kubebuilder:rbac:groups=swift.openstack.org,resources=swifts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=swift.openstack.org,resources=swifts/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=swift.openstack.org,resources=swifts/finalizers,verbs=update

// service account, role, rolebinding
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update
// service account permissions that are needed to grant permission to the above
// +kubebuilder:rbac:groups="security.openshift.io",resourceNames=anyuid;privileged,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:groups="",resources=pods,verbs=create;delete;get;list;patch;update;watch

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
	_ = r.Log.WithValues("swift", req.NamespacedName)

	instance := &swiftv1.Swift{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			r.Log.Info("Swift resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.Log.Error(err, "Failed to get SwiftRing")
		return ctrl.Result{}, err
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

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		// update the Ready condition based on the sub conditions
		if instance.Status.Conditions.AllSubConditionIsTrue() {
			instance.Status.Conditions.MarkTrue(
				condition.ReadyCondition, condition.ReadyMessage)
		} else {
			// something is not ready so reset the Ready condition
			instance.Status.Conditions.MarkUnknown(
				condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage)
			// and recalculate it based on the state of the rest of the conditions
			instance.Status.Conditions.Set(
				instance.Status.Conditions.Mirror(condition.ReadyCondition))
		}
		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	// If we're not deleting this and the service object doesn't have our finalizer, add it.
	if instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, helper.GetFinalizer()) {
		return ctrl.Result{}, nil
	}

	// initialize status

	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
		// initialize conditions used later as Status=Unknown
		cl := condition.CreateList(
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

		// Register overall status immediately to have an early feedback e.g. in the cli
		return ctrl.Result{}, err
	}

	// Handle service delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, instance, helper)
}

func (r *SwiftReconciler) reconcileNormal(ctx context.Context, instance *swiftv1.Swift, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s'", instance.Name))

	// Service account, role, binding
	rbacRules := []rbacv1.PolicyRule{
		{
			APIGroups:     []string{"security.openshift.io"},
			ResourceNames: []string{"anyuid", "privileged"},
			Resources:     []string{"securitycontextconstraints"},
			Verbs:         []string{"use"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"create", "get", "list", "watch", "update", "patch", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
			Verbs:     []string{"create", "get", "update", "delete"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"persistentvolumeclaims"},
			Verbs:     []string{"get", "list", "watch"},
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
			r.Log.Info(fmt.Sprintf("%s... requeueing", condition.MemcachedReadyWaitingMessage))
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
		r.Log.Info(fmt.Sprintf("%s... requeueing", condition.MemcachedReadyWaitingMessage))
		return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
	}
	// Mark the Memcached Service as Ready if we get to this point with no errors
	instance.Status.Conditions.MarkTrue(
		condition.MemcachedReadyCondition, condition.MemcachedReadyMessage)
	// run check memcached - end

	memcachedServers := memcached.GetMemcachedServerListString()

	// create or update Swift storage
	swiftStorage, op, err := r.storageCreateOrUpdate(ctx, instance, memcachedServers)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			swiftv1.SwiftStorageReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			swiftv1.SwiftStorageReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// Mirror SwiftStorage's condition status
	c := swiftStorage.Status.Conditions.Mirror(swiftv1.SwiftStorageReadyCondition)
	if c != nil {
		instance.Status.Conditions.Set(c)
	}

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
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// Mirror SwiftRing's condition status
	c = swiftRing.Status.Conditions.Mirror(swiftv1.SwiftRingReadyCondition)
	if c != nil {
		instance.Status.Conditions.Set(c)
	}

	// create or update Swift proxy
	swiftProxy, op, err := r.proxyCreateOrUpdate(ctx, instance, memcachedServers)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			swiftv1.SwiftProxyReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			swiftv1.SwiftProxyReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// Mirror SwiftProxy's condition status
	c = swiftProxy.Status.Conditions.Mirror(swiftv1.SwiftProxyReadyCondition)
	if c != nil {
		instance.Status.Conditions.Set(c)
	}

	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' successfully", instance.Name))
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
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' delete", instance.Name))

	// Service is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	r.Log.Info(fmt.Sprintf("Reconciled Service '%s' delete successfully", instance.Name))

	return ctrl.Result{}, nil
}

func (r *SwiftReconciler) ringCreateOrUpdate(ctx context.Context, instance *swiftv1.Swift) (*swiftv1.SwiftRing, controllerutil.OperationResult, error) {

	swiftRingSpec := swiftv1.SwiftRingSpec{
		ContainerImage: instance.Spec.SwiftRing.ContainerImage,
		SwiftRingSpecCore: swiftv1.SwiftRingSpecCore{
			RingReplicas: instance.Spec.SwiftRing.RingReplicas,
			TLS:          instance.Spec.SwiftProxy.TLS.Ca,
		},
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

func (r *SwiftReconciler) storageCreateOrUpdate(ctx context.Context, instance *swiftv1.Swift, memcachedServers string) (*swiftv1.SwiftStorage, controllerutil.OperationResult, error) {

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
			MemcachedServers:        memcachedServers,
			ContainerSharderEnabled: instance.Spec.SwiftStorage.ContainerSharderEnabled,
			DefaultConfigOverwrite:  instance.Spec.SwiftStorage.DefaultConfigOverwrite,
		},
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

func (r *SwiftReconciler) proxyCreateOrUpdate(ctx context.Context, instance *swiftv1.Swift, memcachedServers string) (*swiftv1.SwiftProxy, controllerutil.OperationResult, error) {

	swiftProxySpec := swiftv1.SwiftProxySpec{
		ContainerImageProxy: instance.Spec.SwiftProxy.ContainerImageProxy,
		SwiftProxySpecCore: swiftv1.SwiftProxySpecCore{
			Replicas:               instance.Spec.SwiftProxy.Replicas,
			Secret:                 instance.Spec.SwiftProxy.Secret,
			ServiceUser:            instance.Spec.SwiftProxy.ServiceUser,
			PasswordSelectors:      instance.Spec.SwiftProxy.PasswordSelectors,
			Override:               instance.Spec.SwiftProxy.Override,
			NetworkAttachments:     instance.Spec.SwiftProxy.NetworkAttachments,
			MemcachedServers:       memcachedServers,
			TLS:                    instance.Spec.SwiftProxy.TLS,
			DefaultConfigOverwrite: instance.Spec.SwiftProxy.DefaultConfigOverwrite,
			EncryptionEnabled:      instance.Spec.SwiftProxy.EncryptionEnabled,
		},
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
