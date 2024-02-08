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
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/pkg/swift"
)

//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch

func DeviceList(ctx context.Context, h *helper.Helper, instance *swiftv1beta1.SwiftStorage) string {
	// Creates a CSV list of devices. If PVCs do not exist yet (because not
	// all StatefulSets are up yet), it will just use the request capacity
	// as value.
	var devices strings.Builder

	foundClaim := &corev1.PersistentVolumeClaim{}
	for replica := 0; replica < int(*instance.Spec.Replicas); replica++ {
		cn := fmt.Sprintf("%s-%s-%d", swift.ClaimName, instance.Name, replica)
		err := h.GetClient().Get(ctx, types.NamespacedName{Name: cn, Namespace: instance.Namespace}, foundClaim)
		capacity := resource.MustParse(instance.Spec.StorageRequest)
		weight, _ := capacity.AsInt64()
		if err == nil {
			capacity := foundClaim.Status.Capacity["storage"]
			weight, _ = capacity.AsInt64()
		} else {
			h.GetLogger().Info(fmt.Sprintf("Did not find PVC %s, assuming %s as capacity", cn, instance.Spec.StorageRequest))
		}
		weight = weight / (1000 * 1000 * 1000) // 10GiB gets a weight of 10 etc.
		// CSV: region,zone,hostname,devicename,weight
		devices.WriteString(fmt.Sprintf("1,1,%s-%d.%s.%s.svc,%s,%d\n", instance.Name, replica, instance.Name, instance.Namespace, "d1", weight))
	}
	return devices.String()
}

func Labels() map[string]string {
	return map[string]string{"app.kubernetes.io/name": "SwiftStorage"}
}
