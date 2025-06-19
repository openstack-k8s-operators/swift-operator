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
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/env"

	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/pkg/swift"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetRingJob(instance *swiftv1beta1.SwiftRing, labels map[string]string) *batchv1.Job {
	securityContext := swift.GetSecurityContext()

	envVars := map[string]env.Setter{}
	envVars["CM_NAME"] = env.SetValue(instance.Spec.RingConfigMaps[0])
	envVars["NAMESPACE"] = env.SetValue(instance.Namespace)
	envVars["SWIFT_PART_POWER"] = env.SetValue(fmt.Sprint(*instance.Spec.PartPower))
	envVars["SWIFT_REPLICAS"] = env.SetValue(fmt.Sprint(*instance.Spec.RingReplicas))
	envVars["SWIFT_MIN_PART_HOURS"] = env.SetValue(fmt.Sprint(*instance.Spec.MinPartHours))
	envVars["OWNER_APIVERSION"] = env.SetValue(instance.APIVersion)
	envVars["OWNER_KIND"] = env.SetValue(instance.Kind)
	envVars["OWNER_UID"] = env.SetValue(string(instance.ObjectMeta.UID))
	envVars["OWNER_NAME"] = env.SetValue(instance.ObjectMeta.Name)

	volumes := getRingVolumes(instance)
	volumeMounts := getRingVolumeMounts()

	// add CA cert if defined
	if instance.Spec.TLS.CaBundleSecretName != "" {
		volumes = append(volumes, instance.Spec.TLS.CreateVolume())
		volumeMounts = append(volumeMounts, instance.Spec.TLS.CreateVolumeMounts(nil)...)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-rebalance",
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      "OnFailure",
					ServiceAccountName: swift.ServiceAccount,
					SecurityContext: &corev1.PodSecurityContext{
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:            instance.Name + "-rebalance",
							Command:         []string{"/usr/local/bin/swift-ring-tool", "all"},
							Image:           instance.Spec.ContainerImage,
							SecurityContext: &securityContext,
							VolumeMounts:    volumeMounts,
							Env:             env.MergeEnvs([]corev1.EnvVar{}, envVars),
							WorkingDir:      "/etc/swift",
						},
					},
					Volumes: volumes,
				},
			},
		},
	}

	if instance.Spec.NodeSelector != nil {
		job.Spec.Template.Spec.NodeSelector = *instance.Spec.NodeSelector
	}

	return job
}
