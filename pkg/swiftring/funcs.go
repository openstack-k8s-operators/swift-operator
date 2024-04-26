/*

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
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"

	dataplanev1 "github.com/openstack-k8s-operators/dataplane-operator/api/v1beta1"
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/pkg/swift"
)

//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch
//+kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplanenodesets,verbs=get;list;watch

func DeviceList(ctx context.Context, h *helper.Helper, instance *swiftv1beta1.SwiftRing) (string, string, error) {
	// Returns a list of devices as CSV
	devices := []string{}

	listOpts := []client.ListOption{client.InNamespace(instance.GetNamespace())}

	// Get all SwiftStorage instances
	storages := &swiftv1beta1.SwiftStorageList{}
	err := h.GetClient().List(context.Background(), storages, listOpts...)
	if err != nil && !errors.IsNotFound(err) {
		return "", "", err
	}
	for _, storageInstance := range storages.Items {
		for replica := 0; replica < int(*storageInstance.Spec.Replicas); replica++ {
			cn := fmt.Sprintf("%s-%s-%d", swift.ClaimName, storageInstance.Name, replica)
			foundClaim := &corev1.PersistentVolumeClaim{}
			err = h.GetClient().Get(ctx, types.NamespacedName{Name: cn, Namespace: storageInstance.Namespace}, foundClaim)
			if err != nil {
				return "", "", err // requeueing
			}

			if foundClaim.Status.Phase != corev1.ClaimBound {
				err = fmt.Errorf("PersistentVolumeClaim %s found, but not bound yet (%s). Requeueing", cn, foundClaim.Status.Phase)
				return "", "", err // requeueing
			}
			capacity := foundClaim.Status.Capacity[corev1.ResourceStorage]
			weight, _ := capacity.AsInt64()
			weight = weight / (1000 * 1000 * 1000) // 10GiB gets a weight of 10 etc.

			podName := fmt.Sprintf("%s-%d", storageInstance.Name, replica)
			foundPod := &corev1.Pod{}
			err = h.GetClient().Get(ctx, types.NamespacedName{Name: podName, Namespace: storageInstance.Namespace}, foundPod)
			if err != nil {
				err = fmt.Errorf("Pod %s not found", podName)
				return "", "", err
			}
			if foundPod.Spec.NodeName == "" {
				err = fmt.Errorf("Pod %s found, but NodeName not yet set. Requeueing", podName)
				return "", "", err // requeueing
			}

			// Format: region zone hostname devicename weight nodename
			devices = append(devices, fmt.Sprintf("1 1 %s-%d.%s.%s.svc pv %d %s\n", storageInstance.Name, replica, storageInstance.Name, storageInstance.Namespace, weight, foundPod.Spec.NodeName))
		}
	}

	// Get all OpenStackDataPlaneNodeSets that deploy the Swift service and
	// their used Swift disks
	nodeSets := &dataplanev1.OpenStackDataPlaneNodeSetList{}
	err = h.GetClient().List(context.Background(), nodeSets, listOpts...)
	if err != nil && !errors.IsNotFound(err) {
		return "", "", err
	}
	for _, nodeSet := range nodeSets.Items {
		for _, service := range nodeSet.Spec.Services {
			if service == "swift" {
				// Get the global disk vars first that are used for all
				// nodes if not set otherwise per-node
				var globalDisks []swiftv1beta1.SwiftDisk
				if edpmSwiftDisks, found := nodeSet.Spec.NodeTemplate.Ansible.AnsibleVars[DataplaneDisks]; found {
					err = json.Unmarshal(edpmSwiftDisks, &globalDisks)
					if err != nil {
						return "", "", err
					}
				}

				for _, node := range nodeSet.Spec.Nodes {
					hostName := fmt.Sprintf("%s.%s", node.HostName, DataplaneDomain)
					hostDisks := make([]swiftv1beta1.SwiftDisk, len(globalDisks))
					copy(hostDisks, globalDisks)

					// These overwrite the global vars if set
					if edpmSwiftDisks, found := node.Ansible.AnsibleVars[DataplaneDisks]; found {
						hostDisks = nil // clear global disks, per-node settings prevail
						err = json.Unmarshal(edpmSwiftDisks, &hostDisks)
						if err != nil {
							return "", "", err
						}

					}
					for _, disk := range hostDisks {
						devices = append(devices, fmt.Sprintf("%d %d %s %s %d\n", disk.Region, disk.Zone, hostName, filepath.Base(disk.Path), disk.Weight))
					}
				}
			}
		}
	}

	// Device list must be sorted to ensure hash does not change
	sort.Strings(devices)

	var deviceList strings.Builder
	for _, line := range devices {
		deviceList.WriteString(line)
	}

	deviceListHash, err := util.ObjectHash(deviceList.String())
	if err != nil {
		return "", "", err
	}

	return deviceList.String(), deviceListHash, nil
}

func Labels() map[string]string {
	return map[string]string{
		"job-name": "swift-ring-rebalance",
	}
}
