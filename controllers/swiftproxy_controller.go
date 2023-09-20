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
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/deployment"
	"github.com/openstack-k8s-operators/lib-common/modules/common/endpoint"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	service "github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"

	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/pkg/swift"
	"github.com/openstack-k8s-operators/swift-operator/pkg/swiftproxy"
)

// SwiftProxyReconciler reconciles a SwiftProxy object
type SwiftProxyReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Log     logr.Logger
	Kclient kubernetes.Interface
}

//+kubebuilder:rbac:groups=swift.openstack.org,resources=swiftproxies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=swift.openstack.org,resources=swiftproxies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=swift.openstack.org,resources=swiftproxies/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch
//+kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneendpoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SwiftProxy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *SwiftProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("swiftproxy", req.NamespacedName)

	instance := &swiftv1beta1.SwiftProxy{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			r.Log.Info("SwiftProxy resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.Log.Error(err, "Failed to get SwiftProxy")
		return ctrl.Result{}, err
	}

	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
		cl := condition.CreateList(
			condition.UnknownCondition(condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage),
			condition.UnknownCondition(swiftv1beta1.SwiftProxyReadyCondition, condition.InitReason, swiftv1beta1.SwiftProxyReadyInitMessage),
		)

		instance.Status.Conditions.Init(&cl)

		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	helper, err := helper.NewHelper(instance, r.Client, r.Kclient, r.Scheme, r.Log)
	if err != nil {
		return ctrl.Result{}, err
	}

	controllerutil.AddFinalizer(instance, helper.GetFinalizer())
	if err := r.Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, helper)
	}

	serviceLabels := swiftproxy.Labels()

	// Create a Service and endpoints for the proxy
	var swiftPorts = map[service.Endpoint]endpoint.Data{
		service.EndpointPublic: {
			Port: swift.ProxyPort,
			Path: "/v1/AUTH_%(tenant_id)s",
		},
		service.EndpointInternal: {
			Port: swift.ProxyPort,
			Path: "/v1/AUTH_%(tenant_id)s",
		},
	}

	apiEndpoints := make(map[string]string)

	for endpointType, data := range swiftPorts {
		endpointTypeStr := string(endpointType)
		endpointName := swift.ServiceName + "-" + endpointTypeStr
		svcOverride := instance.Spec.Override.Service[endpointType]
		if svcOverride.EmbeddedLabelsAnnotations == nil {
			svcOverride.EmbeddedLabelsAnnotations = &service.EmbeddedLabelsAnnotations{}
		}

		exportLabels := util.MergeStringMaps(
			serviceLabels,
			map[string]string{
				service.AnnotationEndpointKey: endpointTypeStr,
			},
		)

		// Create the service
		svc, err := service.NewService(
			service.GenericService(&service.GenericServiceDetails{
				Name:      endpointName,
				Namespace: instance.Namespace,
				Labels:    exportLabels,
				Selector:  serviceLabels,
				Port: service.GenericServicePort{
					Name:     endpointName,
					Port:     data.Port,
					Protocol: corev1.ProtocolTCP,
				},
			}),
			5,
			&svcOverride.OverrideSpec,
		)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.ExposeServiceReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.ExposeServiceReadyErrorMessage,
				err.Error()))

			return ctrl.Result{}, err
		}

		svc.AddAnnotation(map[string]string{
			service.AnnotationEndpointKey: endpointTypeStr,
		})

		// add Annotation to whether creating an ingress is required or not
		if endpointType == service.EndpointPublic && svc.GetServiceType() == corev1.ServiceTypeClusterIP {
			svc.AddAnnotation(map[string]string{
				service.AnnotationIngressCreateKey: "true",
			})
		} else {
			svc.AddAnnotation(map[string]string{
				service.AnnotationIngressCreateKey: "false",
			})
			if svc.GetServiceType() == corev1.ServiceTypeLoadBalancer {
				svc.AddAnnotation(map[string]string{
					service.AnnotationHostnameKey: svc.GetServiceHostname(), // add annotation to register service name in dnsmasq
				})
			}
		}

		ctrlResult, err := svc.CreateOrPatch(ctx, helper)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.ExposeServiceReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.ExposeServiceReadyErrorMessage,
				err.Error()))

			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.ExposeServiceReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.ExposeServiceReadyRunningMessage))
			return ctrlResult, nil
		}
		// create service - end

		// TODO: TLS, pass in https as protocol, create TLS cert
		apiEndpoints[string(endpointType)], err = svc.GetAPIEndpoint(
			svcOverride.EndpointURL, data.Protocol, data.Path)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	instance.Status.Conditions.MarkTrue(condition.ExposeServiceReadyCondition, condition.ExposeServiceReadyMessage)

	// Create Keystone Service
	serviceSpec := keystonev1.KeystoneServiceSpec{
		ServiceType:        swift.ServiceType,
		ServiceName:        swift.ServiceName,
		ServiceDescription: swift.ServiceDescription,
		Enabled:            true,
		ServiceUser:        instance.Spec.ServiceUser,
		Secret:             instance.Spec.Secret,
		PasswordSelector:   instance.Spec.PasswordSelectors.Service,
	}
	keystoneService := keystonev1.NewKeystoneService(serviceSpec, instance.Namespace, serviceLabels, 10*time.Second)
	ctrlResult, err := keystoneService.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	}

	// Create Keystone endpoints
	endpointSpec := keystonev1.KeystoneEndpointSpec{
		ServiceName: swift.ServiceName,
		Endpoints:   apiEndpoints,
	}
	keystoneEndpoint := keystonev1.NewKeystoneEndpoint(
		swift.ServiceName,
		instance.Namespace,
		endpointSpec,
		serviceLabels,
		10)
	ctrlResult, err = keystoneEndpoint.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	}

	// Get the Keystone authURL
	keystoneAPI, err := keystonev1.GetKeystoneAPI(ctx, helper, instance.Namespace, map[string]string{})
	if err != nil {
		return ctrlResult, err
	}
	authURL, err := keystoneAPI.GetEndpoint(endpoint.EndpointPublic)
	if err != nil {
		return ctrlResult, err
	}

	// Get the service password
	sps, _, err := secret.GetSecret(ctx, helper, instance.Spec.Secret, instance.Namespace)
	if err != nil {
		return ctrlResult, err
	}
	password := string(sps.Data[instance.Spec.PasswordSelectors.Service])

	// Create a Secret populated with content from templates/
	envVars := make(map[string]env.Setter)
	tpl := swiftproxy.SecretTemplates(instance, serviceLabels, authURL, password)
	err = secret.EnsureSecrets(ctx, helper, instance, tpl, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create Deployment
	depl := deployment.NewDeployment(swiftproxy.Deployment(instance, serviceLabels), 5*time.Second)
	ctrlResult, err = depl.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	instance.Status.ReadyCount = depl.GetDeployment().Status.ReadyReplicas
	if instance.Status.ReadyCount > 0 {
		instance.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)
		instance.Status.Conditions.MarkTrue(swiftv1beta1.SwiftProxyReadyCondition, condition.ReadyMessage)
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	r.Log.Info(fmt.Sprintf("Reconciled SwiftProxy '%s' successfully", instance.Name))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SwiftProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&swiftv1beta1.SwiftProxy{}).
		Owns(&corev1.Secret{}).
		Owns(&keystonev1.KeystoneService{}).
		Owns(&keystonev1.KeystoneEndpoint{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *SwiftProxyReconciler) reconcileDelete(ctx context.Context, instance *swiftv1beta1.SwiftProxy, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' delete", instance.Name))

	// Remove the finalizer from our KeystoneEndpoint CR
	keystoneEndpoint, err := keystonev1.GetKeystoneEndpointWithName(ctx, helper, swift.ServiceName, instance.Namespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if err == nil {
		controllerutil.RemoveFinalizer(keystoneEndpoint, helper.GetFinalizer())
		if err = helper.GetClient().Update(ctx, keystoneEndpoint); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		util.LogForObject(helper, "Removed finalizer from our KeystoneEndpoint", instance)
	}

	// Remove the finalizer from our KeystoneService CR
	keystoneService, err := keystonev1.GetKeystoneServiceWithName(ctx, helper, swift.ServiceName, instance.Namespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if err == nil {
		controllerutil.RemoveFinalizer(keystoneService, helper.GetFinalizer())
		if err = helper.GetClient().Update(ctx, keystoneService); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		util.LogForObject(helper, "Removed finalizer from our KeystoneService", instance)
	}

	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	if err := r.Update(ctx, instance); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	r.Log.Info(fmt.Sprintf("Reconciled SwiftProxy '%s' delete successfully", instance.Name))

	return ctrl.Result{}, nil
}
