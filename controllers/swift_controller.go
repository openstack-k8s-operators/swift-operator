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

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	common_rbac "github.com/openstack-k8s-operators/lib-common/modules/common/rbac"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
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
func (r *SwiftReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("swift", req.NamespacedName)

	instance := &swiftv1beta1.Swift{}
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

	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
		cl := condition.CreateList(
			condition.UnknownCondition(condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage),
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
			Verbs:     []string{"create", "update", "delete"},
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

	labels := swift.GetLabelsSwift()

	// Create a Secret populated with content from templates/
	_, _, err = secret.GetSecret(ctx, helper, instance.Spec.SwiftConfSecret, instance.Namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			envVars := make(map[string]env.Setter)
			tpl := getSwiftSecretTemplates(instance, labels)
			err = secret.EnsureSecrets(ctx, helper, instance, tpl, &envVars)
			if err != nil {
				return ctrl.Result{}, err
			}
		} else {
			return ctrl.Result{}, err
		}
	}

	// create or update Swift rings
	swiftRing, op, err := r.ringCreateOrUpdate(ctx, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			swiftv1beta1.SwiftRingReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			swiftv1beta1.SwiftRingReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// Mirror SwiftRing's condition status
	c := swiftRing.Status.Conditions.Mirror(swiftv1beta1.SwiftRingReadyCondition)
	if c != nil {
		instance.Status.Conditions.Set(c)
	}

	// create or update Swift storage
	swiftStorage, op, err := r.storageCreateOrUpdate(ctx, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			swiftv1beta1.SwiftStorageReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			swiftv1beta1.SwiftStorageReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// Mirror SwiftStorage's condition status
	c = swiftStorage.Status.Conditions.Mirror(swiftv1beta1.SwiftStorageReadyCondition)
	if c != nil {
		instance.Status.Conditions.Set(c)
	}

	// create or update Swift proxy
	swiftProxy, op, err := r.proxyCreateOrUpdate(ctx, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			swiftv1beta1.SwiftProxyReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			swiftv1beta1.SwiftProxyReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled - operation: %s", instance.Name, string(op)))
	}

	// Mirror SwiftProxy's condition status
	c = swiftProxy.Status.Conditions.Mirror(swiftv1beta1.SwiftProxyReadyCondition)
	if c != nil {
		instance.Status.Conditions.Set(c)
	}

	if instance.IsReady() {
		instance.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}

		r.Log.Info(fmt.Sprintf("Deployment %s successfully reconciled", instance.Name))
	}

	return ctrl.Result{}, nil
}

func getSwiftSecretTemplates(instance *swiftv1beta1.Swift, labels map[string]string) []util.Template {
	templateParameters := make(map[string]interface{})
	templateParameters["SwiftHashPathPrefix"] = swift.RandomString(16)
	templateParameters["SwiftHashPathSuffix"] = swift.RandomString(16)

	return []util.Template{
		{
			Name:          instance.Spec.SwiftConfSecret,
			Namespace:     instance.Namespace,
			Type:          util.TemplateTypeConfig,
			InstanceType:  instance.Kind,
			ConfigOptions: templateParameters,
			Labels:        labels,
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *SwiftReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&swiftv1beta1.Swift{}).
		Owns(&swiftv1beta1.SwiftRing{}).
		Owns(&swiftv1beta1.SwiftStorage{}).
		Owns(&swiftv1beta1.SwiftProxy{}).
		Complete(r)
}

func (r *SwiftReconciler) ringCreateOrUpdate(ctx context.Context, instance *swiftv1beta1.Swift) (*swiftv1beta1.SwiftRing, controllerutil.OperationResult, error) {

	swiftRingSpec := swiftv1beta1.SwiftRingSpec{
		RingConfigMap:   instance.Spec.RingConfigMap,
		RingReplicas:    instance.Spec.SwiftRing.RingReplicas,
		ContainerImage:  instance.Spec.SwiftRing.ContainerImage,
		SwiftConfSecret: instance.Spec.SwiftConfSecret,
	}

	deployment := &swiftv1beta1.SwiftRing{
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

func (r *SwiftReconciler) storageCreateOrUpdate(ctx context.Context, instance *swiftv1beta1.Swift) (*swiftv1beta1.SwiftStorage, controllerutil.OperationResult, error) {

	swiftStorageSpec := swiftv1beta1.SwiftStorageSpec{
		Replicas:                instance.Spec.SwiftStorage.Replicas,
		RingConfigMap:           instance.Spec.RingConfigMap,
		StorageClass:            instance.Spec.SwiftStorage.StorageClass,
		StorageRequest:          instance.Spec.SwiftStorage.StorageRequest,
		ContainerImageAccount:   instance.Spec.SwiftStorage.ContainerImageAccount,
		ContainerImageContainer: instance.Spec.SwiftStorage.ContainerImageContainer,
		ContainerImageObject:    instance.Spec.SwiftStorage.ContainerImageObject,
		ContainerImageProxy:     instance.Spec.SwiftStorage.ContainerImageProxy,
		ContainerImageMemcached: instance.Spec.SwiftStorage.ContainerImageMemcached,
		SwiftConfSecret:         instance.Spec.SwiftConfSecret,
	}

	deployment := &swiftv1beta1.SwiftStorage{
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

func (r *SwiftReconciler) proxyCreateOrUpdate(ctx context.Context, instance *swiftv1beta1.Swift) (*swiftv1beta1.SwiftProxy, controllerutil.OperationResult, error) {

	swiftProxySpec := swiftv1beta1.SwiftProxySpec{
		Replicas:                instance.Spec.SwiftProxy.Replicas,
		RingConfigMap:           instance.Spec.RingConfigMap,
		ContainerImageProxy:     instance.Spec.SwiftProxy.ContainerImageProxy,
		ContainerImageMemcached: instance.Spec.SwiftProxy.ContainerImageMemcached,
		Secret:                  instance.Spec.SwiftProxy.Secret,
		ServiceUser:             instance.Spec.SwiftProxy.ServiceUser,
		PasswordSelectors:       instance.Spec.SwiftProxy.PasswordSelectors,
		SwiftConfSecret:         instance.Spec.SwiftConfSecret,
	}

	deployment := &swiftv1beta1.SwiftProxy{
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
