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
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	env "github.com/openstack-k8s-operators/lib-common/modules/common/env"
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/internal/swift"
)

func getPorts(port int32, name string) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			ContainerPort: port,
			Name:          name,
		},
	}
}

func getStorageContainers(swiftstorage *swiftv1beta1.SwiftStorage, env []corev1.EnvVar, volumeMounts []corev1.VolumeMount) []corev1.Container {
	securityContext := swift.GetSecurityContext()

	containers := []corev1.Container{
		{
			Name:            "account-server",
			Image:           swiftstorage.Spec.ContainerImageAccount,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			Ports:           getPorts(swift.AccountServerPort, "account"),
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/swift-account-server", "/etc/swift/account-server.conf.d", "-v"},
			Env:             env,
			Resources:       swiftstorage.Spec.Resources,
		},
		{
			Name:            "account-replicator",
			Image:           swiftstorage.Spec.ContainerImageAccount,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/swift-account-replicator", "/etc/swift/account-server.conf.d", "-v"},
			Env:             env,
		},
		{
			Name:            "account-auditor",
			Image:           swiftstorage.Spec.ContainerImageAccount,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/swift-account-auditor", "/etc/swift/account-server.conf.d", "-v"},
			Env:             env,
		},
		{
			Name:            "account-reaper",
			Image:           swiftstorage.Spec.ContainerImageAccount,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/swift-account-reaper", "/etc/swift/account-server.conf.d", "-v"},
			Env:             env,
		},
		{
			Name:            "container-server",
			Image:           swiftstorage.Spec.ContainerImageContainer,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			Ports:           getPorts(swift.ContainerServerPort, "container"),
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/swift-container-server", "/etc/swift/container-server.conf.d", "-v"},
			Env:             env,
			Resources:       swiftstorage.Spec.Resources,
		},
		{
			Name:            "container-replicator",
			Image:           swiftstorage.Spec.ContainerImageContainer,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/swift-container-replicator", "/etc/swift/container-server.conf.d", "-v"},
			Env:             env,
		},
		{
			Name:            "container-auditor",
			Image:           swiftstorage.Spec.ContainerImageContainer,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/swift-container-auditor", "/etc/swift/container-server.conf.d", "-v"},
			Env:             env,
		},
		{
			Name:            "container-updater",
			Image:           swiftstorage.Spec.ContainerImageContainer,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/swift-container-updater", "/etc/swift/container-server.conf.d", "-v"},
			Env:             env,
		},
		{
			Name:            "object-server",
			Image:           swiftstorage.Spec.ContainerImageObject,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			Ports:           getPorts(swift.ObjectServerPort, "object"),
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/swift-object-server", "/etc/swift/object-server.conf.d", "-v"},
			Env:             env,
			Resources:       swiftstorage.Spec.Resources,
		},
		{
			Name:            "object-replicator",
			Image:           swiftstorage.Spec.ContainerImageObject,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/swift-object-replicator", "/etc/swift/object-server.conf.d", "-v"},
			Env:             env,
		},
		{
			Name:            "object-auditor",
			Image:           swiftstorage.Spec.ContainerImageObject,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/swift-object-auditor", "/etc/swift/object-server.conf.d", "-v"},
			Env:             env,
		},
		{
			Name:            "object-updater",
			Image:           swiftstorage.Spec.ContainerImageObject,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/swift-object-updater", "/etc/swift/object-server.conf.d", "-v"},
			Env:             env,
		},
		{
			Name:            "object-expirer",
			Image:           swiftstorage.Spec.ContainerImageProxy,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/swift-object-expirer", "/etc/swift/object-expirer.conf.d", "-v"},
			Env:             env,
		},
		{
			Name:            "rsync",
			Image:           swiftstorage.Spec.ContainerImageObject,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			Ports:           getPorts(swift.RsyncPort, "rsync"),
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/rsync", "--daemon", "--no-detach", "--config=/etc/swift/rsyncd.conf", "--log-file=/dev/stdout"},
			Env:             env,
		},
		{
			Name:            "swift-recon-cron",
			Image:           swiftstorage.Spec.ContainerImageObject,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			VolumeMounts:    volumeMounts,
			Command:         []string{"sh", "-c", "while true; do /usr/bin/swift-recon-cron /etc/swift/object-server.conf.d -v; sleep 300; done"},
			Env:             env,
		},
	}

	if swiftstorage.Spec.ContainerSharderEnabled {
		containers = append(containers, corev1.Container{
			Name:            "container-sharder",
			Image:           swiftstorage.Spec.ContainerImageContainer,
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &securityContext,
			VolumeMounts:    volumeMounts,
			Command:         []string{"/usr/bin/swift-container-sharder", "/etc/swift/container-server.conf.d", "-v"},
			Env:             env,
		})
	}

	return containers
}

// StatefulSet creates a Kubernetes StatefulSet for swift storage nodes
func StatefulSet(
	swiftstorage *swiftv1beta1.SwiftStorage,
	labels map[string]string,
	annotations map[string]string,
	configHash string,
	topology *topologyv1.Topology,
	memcached *memcachedv1.Memcached,
) (*appsv1.StatefulSet, error) {
	trueVal := true
	OnRootMismatch := corev1.FSGroupChangeOnRootMismatch
	user := int64(swift.RunAsUser)

	envVars := map[string]env.Setter{}
	envVars["CONFIG_HASH"] = env.SetValue(configHash)
	env := env.MergeEnvs([]corev1.EnvVar{}, envVars)
	// if swiftstorage.Spec.StorageRequest is not a valid k8s Quantity, return
	// an error
	pvcSize, err := resource.ParseQuantity(swiftstorage.Spec.StorageRequest)
	if err != nil {
		return nil, err
	}

	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      swiftstorage.Name,
			Namespace: swiftstorage.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: swiftstorage.Name,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas:            swiftstorage.Spec.Replicas,
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: swift.ServiceAccount,
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup:             &user,
						FSGroupChangePolicy: &OnRootMismatch,
						Sysctls: []corev1.Sysctl{{
							Name:  "net.ipv4.ip_unprivileged_port_start",
							Value: "873",
						}},
						RunAsNonRoot: &trueVal,
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Volumes:    nil, // Will be set below after adding CA bundle and MTLS volumes
					Containers: nil, // Will be set below after creating volumeMounts
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name: swift.ClaimName,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &swiftstorage.Spec.StorageClass,
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: pvcSize,
						},
					},
				},
			}},
		},
	}

	// Create volumes and volumeMounts
	volumes := getStorageVolumes(swiftstorage)
	volumeMounts := getStorageVolumeMounts()

	// Add general CA cert if defined
	if swiftstorage.Spec.TLS.CaBundleSecretName != "" {
		volumes = append(volumes, swiftstorage.Spec.TLS.CreateVolume())
		volumeMounts = append(volumeMounts, swiftstorage.Spec.TLS.CreateVolumeMounts(nil)...)
	}

	// Add Memcached MTLS certs if available
	if memcached.Status.MTLSCert != "" {
		volumes = append(volumes, memcached.CreateMTLSVolume())
		volumeMounts = append(volumeMounts, memcached.CreateMTLSVolumeMounts(ptr.To(memcachedv1.CertPathDst), ptr.To(memcachedv1.KeyPathDst))...)
	}

	// Set volumes and containers on the StatefulSet
	statefulset.Spec.Template.Spec.Volumes = volumes
	statefulset.Spec.Template.Spec.Containers = getStorageContainers(swiftstorage, env, volumeMounts)

	if swiftstorage.Spec.NodeSelector != nil {
		statefulset.Spec.Template.Spec.NodeSelector = *swiftstorage.Spec.NodeSelector
	}

	if topology != nil {
		topology.ApplyTo(&statefulset.Spec.Template)
	} else {
		// If possible two pods of the same service should not
		// run on the same worker node. If this is not possible
		// the get still created on the same worker node.
		statefulset.Spec.Template.Spec.Affinity = swift.GetPodAffinity(ComponentName)
	}

	return statefulset, nil
}
