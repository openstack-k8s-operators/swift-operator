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

package swiftstorage

import (
	corev1 "k8s.io/api/core/v1"

	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/pkg/swift"
)

func getStorageVolumes(instance *swiftv1beta1.SwiftStorage) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: swift.ClaimName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: swift.ClaimName,
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
							ConfigMap: &corev1.ConfigMapProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: instance.Name + "-config-data",
								},
								Items: []corev1.KeyToPath{
									{
										Key:  "00-account-server.conf",
										Path: "account-server.conf.d/00-account-server.conf",
									},
									{
										Key:  "01-account-server.conf",
										Path: "account-server.conf.d/01-account-server.conf",
									},
									{
										Key:  "00-container-server.conf",
										Path: "container-server.conf.d/00-container-server.conf",
									},
									{
										Key:  "01-container-server.conf",
										Path: "container-server.conf.d/01-container-server.conf",
									},
									{
										Key:  "00-object-server.conf",
										Path: "object-server.conf.d/00-object-server.conf",
									},
									{
										Key:  "01-object-server.conf",
										Path: "object-server.conf.d/01-object-server.conf",
									},
									{
										Key:  "00-object-expirer.conf",
										Path: "object-expirer.conf.d/00-object-expirer.conf",
									},
									{
										Key:  "01-object-expirer.conf",
										Path: "object-expirer.conf.d/01-object-expirer.conf",
									},
									{
										Key:  "internal-client.conf",
										Path: "internal-client.conf",
									},
									{
										Key:  "rsyncd.conf",
										Path: "rsyncd.conf",
									},
								},
							},
						},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: instance.Spec.SwiftConfSecret,
								},
							},
						},
					},
				},
			},
		},
		{
			Name: "cache",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{Medium: ""},
			},
		},
		{
			Name: "lock",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{Medium: ""},
			},
		},
	}
}

func getStorageVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      swift.ClaimName,
			MountPath: "/srv/node/d1",
			ReadOnly:  false,
		},
		{
			Name:      "etc-swift",
			MountPath: "/etc/swift",
			ReadOnly:  false,
		},
		{
			Name:      "cache",
			MountPath: "/var/cache/swift",
			ReadOnly:  false,
		},
		{
			Name:      "lock",
			MountPath: "/var/lock",
			ReadOnly:  false,
		},
	}
}
