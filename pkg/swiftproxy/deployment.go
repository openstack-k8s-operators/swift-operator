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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/intstr"

	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	swift "github.com/openstack-k8s-operators/swift-operator/pkg/swift"
)

func Deployment(
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
					Volumes: getProxyVolumes(instance),
					Containers: []corev1.Container{
						{
							Name:            "ring-sync",
							Image:           instance.Spec.ContainerImageProxy,
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &securityContext,
							ReadinessProbe:  readinessProbe,
							LivenessProbe:   livenessProbe,
							VolumeMounts:    getProxyVolumeMounts(),
							Command:         []string{"/usr/local/bin/container-scripts/ring-sync.sh"},
						},
						{
							Image:           instance.Spec.ContainerImageProxy,
							Name:            "proxy-server",
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &securityContext,
							Ports: []corev1.ContainerPort{{
								ContainerPort: swift.ProxyPort,
								Name:          "proxy-server",
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
