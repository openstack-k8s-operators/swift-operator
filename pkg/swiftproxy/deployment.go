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

package swiftproxy

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	swift "github.com/openstack-k8s-operators/swift-operator/pkg/swift"
)

func Deployment(
	instance *swiftv1beta1.SwiftProxy,
	labels map[string]string,
	annotations map[string]string,
	configHash string,
) (*appsv1.Deployment, error) {

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
		Port: intstr.FromInt(int(swift.ProxyHttpdPort)),
	}
	readinessProbe.HTTPGet = &corev1.HTTPGetAction{
		Path: "/healthcheck",
		Port: intstr.FromInt(int(swift.ProxyHttpdPort)),
	}

	if instance.Spec.TLS.API.Enabled(service.EndpointPublic) {
		livenessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
		readinessProbe.HTTPGet.Scheme = corev1.URISchemeHTTPS
	}

	// create Volume and VolumeMounts
	volumes := getProxyVolumes(instance)
	volumeMounts := getProxyVolumeMounts()
	httpdVolumeMounts := append(getProxyVolumeMounts(), getHttpdVolumeMounts()...)

	// add CA cert if defined
	if instance.Spec.TLS.CaBundleSecretName != "" {
		volumes = append(volumes, instance.Spec.TLS.CreateVolume())
		volumeMounts = append(volumeMounts, instance.Spec.TLS.CreateVolumeMounts(nil)...)
		httpdVolumeMounts = append(httpdVolumeMounts, instance.Spec.TLS.CreateVolumeMounts(nil)...)
	}

	for _, endpt := range []service.Endpoint{service.EndpointInternal, service.EndpointPublic} {
		if instance.Spec.TLS.API.Enabled(endpt) {
			var tlsEndptCfg tls.GenericService
			switch endpt {
			case service.EndpointPublic:
				tlsEndptCfg = instance.Spec.TLS.API.Public
			case service.EndpointInternal:
				tlsEndptCfg = instance.Spec.TLS.API.Internal
			}

			svc, err := tlsEndptCfg.ToService()
			if err != nil {
				return nil, err
			}
			// httpd container is not using kolla, mount the certs to its dst
			svc.CertMount = ptr.To(fmt.Sprintf("/etc/pki/tls/certs/%s.crt", endpt.String()))
			svc.KeyMount = ptr.To(fmt.Sprintf("/etc/pki/tls/private/%s.key", endpt.String()))

			volumes = append(volumes, svc.CreateVolume(endpt.String()))
			httpdVolumeMounts = append(httpdVolumeMounts, svc.CreateVolumeMounts(endpt.String())...)
		}
	}

	envVars := map[string]env.Setter{}
	envVars["CONFIG_HASH"] = env.SetValue(configHash)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: instance.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: swift.ServiceAccount,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &trueVal,
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
						// httpd needs to have access to the certificates in /etc/pki/tls/certs/...
						// which per default is root/root/0400, setting the FSGroup results in everything
						// mounted to the pod to have the swift group set, now the certs will be mounted
						// as root/swift/0440.
						FSGroup: ptr.To(swift.RunAsUser),
					},
					Volumes: volumes,
					Containers: []corev1.Container{
						{
							Name:            "ring-sync",
							Image:           instance.Spec.ContainerImageProxy,
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &securityContext,
							ReadinessProbe:  readinessProbe,
							LivenessProbe:   livenessProbe,
							VolumeMounts:    volumeMounts,
							Command:         []string{"/usr/local/bin/container-scripts/ring-sync.sh"},
						},
						{
							Image:           instance.Spec.ContainerImageProxy,
							Name:            "proxy-httpd",
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &securityContext,
							Env:             env.MergeEnvs([]corev1.EnvVar{}, envVars),
							Ports: []corev1.ContainerPort{{
								ContainerPort: swift.ProxyHttpdPort,
								Name:          "proxy-httpd",
							}},
							ReadinessProbe:           readinessProbe,
							LivenessProbe:            livenessProbe,
							VolumeMounts:             httpdVolumeMounts,
							Command:                  []string{"/usr/sbin/httpd"},
							Args:                     []string{"-DFOREGROUND"},
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						},
						{
							Image:           instance.Spec.ContainerImageProxy,
							Name:            "proxy-server",
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &securityContext,
							Env:             env.MergeEnvs([]corev1.EnvVar{}, envVars),
							Ports: []corev1.ContainerPort{{
								ContainerPort: swift.ProxyPort,
								Name:          "proxy-server",
							}},
							ReadinessProbe: readinessProbe,
							LivenessProbe:  livenessProbe,
							VolumeMounts:   volumeMounts,
							Command:        []string{"/usr/bin/swift-proxy-server", "/etc/swift/proxy-server.conf", "-v"},
						},
					},
				},
			},
		},
	}, nil
}
