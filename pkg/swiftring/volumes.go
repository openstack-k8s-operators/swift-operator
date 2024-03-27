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

package swiftring

import (
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

func getRingVolumes(instance *swiftv1beta1.SwiftRing) []corev1.Volume {
	var scriptsVolumeDefaultMode int32 = 0755
	trueVal := true
	return []corev1.Volume{
		{
			Name: "scripts",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &scriptsVolumeDefaultMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Name + "-scripts",
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "swift-ring-tool",
							Path: "swift-ring-tool",
						},
					},
				},
			},
		},
		{
			Name: "swiftconf",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: instance.Spec.SwiftConfSecret,
					Items: []corev1.KeyToPath{
						{
							Key:  "swift.conf",
							Path: "swift.conf",
						},
					},
				},
			},
		},
		{
			Name: "etc-swift",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{Medium: ""},
			},
		},
		{
			Name: "ring-data-devices",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Name + "-config-data",
					},
				},
			},
		},
		{
			Name: "dispersionconf",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "swift-proxy-config-data",
					Optional:   &trueVal,
					Items: []corev1.KeyToPath{
						{
							Key:  "dispersion.conf",
							Path: "dispersion.conf",
						},
					},
				},
			},
		},
	}
}

func getRingVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "scripts",
			SubPath:   "swift-ring-tool",
			MountPath: "/usr/local/bin/swift-ring-tool",
			ReadOnly:  true,
		},
		{
			Name:      "swiftconf",
			SubPath:   "swift.conf",
			MountPath: "/etc/swift/swift.conf",
			ReadOnly:  true,
		},
		{
			Name:      "etc-swift",
			MountPath: "/etc/swift",
			ReadOnly:  false,
		},
		{
			Name:      "ring-data-devices",
			MountPath: "/var/lib/config-data/ring-devices",
			ReadOnly:  true,
		},
		{
			Name:      "dispersionconf",
			SubPath:   "dispersion.conf",
			MountPath: "/etc/swift/dispersion.conf",
			ReadOnly:  true,
		},
	}
}
