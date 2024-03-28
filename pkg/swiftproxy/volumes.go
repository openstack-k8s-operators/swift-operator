/*
Copyright 2023.

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
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/pkg/swift"
	corev1 "k8s.io/api/core/v1"
)

func getProxyVolumes(instance *swiftv1beta1.SwiftProxy) []corev1.Volume {
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
			Name: "etc-swift",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{
						{
							ConfigMap: &corev1.ConfigMapProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: swift.RingConfigMapName,
								},
							}},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: instance.Name + "-config-data",
								},
								Items: []corev1.KeyToPath{
									{
										Key:  "00-proxy-server.conf",
										Path: "proxy-server.conf.d/00-proxy-server.conf",
									},
									{
										Key:  "01-proxy-server.conf",
										Path: "proxy-server.conf.d/01-proxy-server.conf",
									},
									{
										Key:  "dispersion.conf",
										Path: "dispersion.conf",
									},
									{
										Key:  "keymaster.conf",
										Path: "keymaster.conf",
									},
								},
							},
						},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: swift.SwiftConfSecretName,
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "run-httpd",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{Medium: ""},
			},
		},
		{
			Name: "log-httpd",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{Medium: ""},
			},
		},
	}
}

func getProxyVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "etc-swift",
			MountPath: "/etc/swift",
			ReadOnly:  false,
		},
	}
}

// getHttpdVolumeMounts - Returns the VolumeMounts used by the httpd sidecar
func getHttpdVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "config-data",
			MountPath: "/etc/httpd/conf/httpd.conf",
			SubPath:   "httpd.conf",
			ReadOnly:  true,
		},
		{
			Name:      "config-data",
			MountPath: "/etc/httpd/conf.d/ssl.conf",
			SubPath:   "ssl.conf",
			ReadOnly:  true,
		},
		{
			Name:      "run-httpd",
			MountPath: "/run/httpd",
			ReadOnly:  false,
		},
		{
			Name:      "log-httpd",
			MountPath: "/var/log/httpd",
			ReadOnly:  false,
		},
	}
}
