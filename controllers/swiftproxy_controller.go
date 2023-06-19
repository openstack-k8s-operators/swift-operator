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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/deployment"
	"github.com/openstack-k8s-operators/lib-common/modules/common/endpoint"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"

	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	swift "github.com/openstack-k8s-operators/swift-operator/pkg/swift"
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
//+kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete;
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
			condition.UnknownCondition(swiftv1beta1.SwiftProxyReadyCondition, condition.InitReason, swiftv1beta1.SwiftRingReadyInitMessage),
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

	// Check if there is a ConfigMap for the Swift rings
	_, ctrlResult, err := configmap.GetConfigMap(
		ctx, helper, instance, instance.Spec.RingConfigMap, 5*time.Second)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	labels := swift.GetLabelsProxy()

	// Create a Service and endpoints for the proxy
	var swiftPorts = map[endpoint.Endpoint]endpoint.Data{
		endpoint.EndpointAdmin: endpoint.Data{
			Port: swift.ProxyPort,
			Path: "",
		},
		endpoint.EndpointPublic: endpoint.Data{
			Port: swift.ProxyPort,
			Path: "/v1/AUTH_%(tenant_id)s",
		},
		endpoint.EndpointInternal: endpoint.Data{
			Port: swift.ProxyPort,
			Path: "/v1/AUTH_%(tenant_id)s",
		},
	}

	apiEndpoints, ctrlResult, err := endpoint.ExposeEndpoints(
		ctx,
		helper,
		swift.ServiceName,
		labels,
		swiftPorts,
		time.Duration(5)*time.Second,
	)
	if err != nil {
		r.Log.Error(err, "Failed to expose endpoints for Swift Proxy")
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	if instance.Status.APIEndpoints == nil {
		instance.Status.APIEndpoints = map[string]map[string]string{}
	}
	instance.Status.APIEndpoints[swift.ServiceName] = apiEndpoints

	// Create Keystone Service
	ksh := getKeystoneServiceHelper(instance, labels)
	ctrlResult, err = ksh.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	}

	// Create Keystone endpoints
	eph := getKeystoneEndpointHelper(instance, labels)
	ctrlResult, err = eph.CreateOrPatch(ctx, helper)
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
	tpl := getProxySecretTemplates(instance, labels, authURL, password)
	err = secret.EnsureSecrets(ctx, helper, instance, tpl, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Create Deployment
	depl := deployment.NewDeployment(getProxyDeployment(instance, labels), 5*time.Second)
	ctrlResult, err = depl.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	if depl.GetDeployment().Status.ReadyReplicas > 0 {
		instance.Status.Conditions.MarkTrue(swiftv1beta1.SwiftProxyReadyCondition, condition.DeploymentReadyMessage)

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
		Owns(&routev1.Route{}).
		Complete(r)
}

func getProxySecretTemplates(instance *swiftv1beta1.SwiftProxy, labels map[string]string, authURL string, password string) []util.Template {
	templateParameters := make(map[string]interface{})
	templateParameters["ServiceUser"] = instance.Spec.ServiceUser
	templateParameters["ServicePassword"] = password
	templateParameters["KeystonePublicURL"] = authURL

	return []util.Template{
		{
			Name:          fmt.Sprintf("%s-config-data", instance.Name),
			Namespace:     instance.Namespace,
			Type:          util.TemplateTypeConfig,
			InstanceType:  instance.Kind,
			ConfigOptions: templateParameters,
			Labels:        labels,
		},
		{
			Name:               fmt.Sprintf("%s-scripts", instance.Name),
			Namespace:          instance.Namespace,
			Type:               util.TemplateTypeScripts,
			AdditionalTemplate: map[string]string{"swift-init.sh": "/common/swift-init.sh"},
			InstanceType:       instance.Kind,
			Labels:             labels,
		},
	}
}

func getProxyVolumes(instance *swiftv1beta1.SwiftProxy) []corev1.Volume {
	var scriptsVolumeDefaultMode int32 = 0755
	return []corev1.Volume{
		{
			Name: "config-data",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: instance.Name + "-config-data",
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
			Name: "ring-data",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Spec.RingConfigMap,
					},
				},
			},
		},
		{
			Name: "config-data-merged",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{Medium: ""},
			},
		},
		{
			Name: "scripts",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					DefaultMode: &scriptsVolumeDefaultMode,
					SecretName:  instance.Name + "-scripts",
				},
			},
		},
	}
}

func getProxyVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "config-data",
			MountPath: "/var/lib/config-data/default",
			ReadOnly:  true,
		},
		{
			Name:      "swiftconf",
			MountPath: "/var/lib/config-data/swiftconf",
			ReadOnly:  true,
		},
		{
			Name:      "ring-data",
			MountPath: "/var/lib/config-data/rings",
			ReadOnly:  true,
		},
		{
			Name:      "config-data-merged",
			MountPath: "/etc/swift",
			ReadOnly:  false,
		},
		{
			Name:      "scripts",
			MountPath: "/usr/local/bin/container-scripts",
			ReadOnly:  true,
		},
	}
}

func getInitContainers(swiftproxy *swiftv1beta1.SwiftProxy) []corev1.Container {
	securityContext := swift.GetSecurityContext()
	return []corev1.Container{
		{
			Name:            "swift-init",
			Image:           swiftproxy.Spec.ContainerImageProxy,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			VolumeMounts:    getProxyVolumeMounts(),
			Command:         []string{"/usr/local/bin/container-scripts/swift-init.sh"},
		},
	}
}

func getProxyDeployment(
	instance *swiftv1beta1.SwiftProxy, labels map[string]string) *appsv1.Deployment {

	trueVal := true
	securityContext := swift.GetSecurityContext()

	livenessProbe := &corev1.Probe{
		TimeoutSeconds:      5,
		PeriodSeconds:       3,
		InitialDelaySeconds: 5,
	}
	readinessProbe := &corev1.Probe{
		TimeoutSeconds:      5,
		PeriodSeconds:       5,
		InitialDelaySeconds: 5,
	}

	livenessProbe.HTTPGet = &corev1.HTTPGetAction{
		Path: "/healthcheck",
		Port: intstr.FromInt(int(swift.ProxyPort)),
	}
	readinessProbe.HTTPGet = &corev1.HTTPGetAction{
		Path: "/healthcheck",
		Port: intstr.FromInt(int(swift.ProxyPort)),
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: &instance.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: swift.ServiceAccount,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &trueVal,
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Volumes:        getProxyVolumes(instance),
					InitContainers: getInitContainers(instance),
					Containers: []corev1.Container{
						{
							Image:           instance.Spec.ContainerImageProxy,
							Name:            "instance",
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &securityContext,
							Ports: []corev1.ContainerPort{{
								ContainerPort: swift.ProxyPort,
								Name:          "instance",
							}},
							ReadinessProbe: readinessProbe,
							LivenessProbe:  livenessProbe,
							VolumeMounts:   getProxyVolumeMounts(),
							Command:        []string{"/usr/bin/swift-proxy-server", "/etc/swift/proxy-server.conf", "-v"},
						},
						{
							Image:           instance.Spec.ContainerImageMemcached,
							Name:            "memcached",
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &securityContext,
							Ports: []corev1.ContainerPort{{
								ContainerPort: swift.MemcachedPort,
								Name:          "memcached",
							}},
							VolumeMounts: getProxyVolumeMounts(),
							Command:      []string{"/usr/bin/memcached", "-p", "11211", "-u", "memcached"},
						},
					},
				},
			},
		},
	}
}

func getKeystoneServiceHelper(
	instance *swiftv1beta1.SwiftProxy, labels map[string]string) *keystonev1.KeystoneServiceHelper {

	spec := keystonev1.KeystoneServiceSpec{
		ServiceType:        swift.ServiceType,
		ServiceName:        swift.ServiceName,
		ServiceDescription: swift.ServiceDescription,
		Enabled:            true,
		ServiceUser:        instance.Spec.ServiceUser,
		Secret:             instance.Spec.Secret,
		PasswordSelector:   instance.Spec.PasswordSelectors.Service,
	}

	return keystonev1.NewKeystoneService(spec, instance.Namespace, labels, 10*time.Second)
}

func getKeystoneEndpointHelper(
	instance *swiftv1beta1.SwiftProxy, labels map[string]string) *keystonev1.KeystoneEndpointHelper {

	spec := keystonev1.KeystoneEndpointSpec{
		ServiceName: swift.ServiceName,
		Endpoints:   instance.Status.APIEndpoints[swift.ServiceName],
	}

	return keystonev1.NewKeystoneEndpoint(
		swift.ServiceName,
		instance.Namespace,
		spec,
		labels,
		10)
}

func (r *SwiftProxyReconciler) reconcileDelete(ctx context.Context, instance *swiftv1beta1.SwiftProxy, helper *helper.Helper) (ctrl.Result, error) {
	r.Log.Info(fmt.Sprintf("Reconciling Service '%s' delete", instance.Name))

	// It's possible to get here before the endpoints have been set in the status, so check for this
	if instance.Status.APIEndpoints != nil {

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
	}

	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	if err := r.Update(ctx, instance); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	r.Log.Info(fmt.Sprintf("Reconciled SwiftProxy '%s' delete successfully", instance.Name))

	return ctrl.Result{}, nil
}
