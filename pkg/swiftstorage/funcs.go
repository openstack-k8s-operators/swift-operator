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
	"k8s.io/apimachinery/pkg/types"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/pkg/swift"
)

//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch

func DeviceList(ctx context.Context, h *helper.Helper, instance *swiftv1beta1.SwiftStorage) (string, error) {
	var devices strings.Builder

	foundClaim := &corev1.PersistentVolumeClaim{}
	for replica := 0; replica < int(instance.Spec.Replicas); replica++ {
		cn := fmt.Sprintf("%s-%s-%d", swift.ClaimName, instance.Name, replica)
		err := h.GetClient().Get(ctx, types.NamespacedName{Name: cn, Namespace: instance.Namespace}, foundClaim)
		if err == nil {
			fsc := foundClaim.Status.Capacity["storage"]
			c, _ := (&fsc).AsInt64()
			c = c / (1000 * 1000 * 1000)
			host := fmt.Sprintf("%s-%d.%s", instance.Name, replica, instance.Name)
			devices.WriteString(fmt.Sprintf("%s,%s,%d\n", host, "d1", c))
		} else {
			return "", err
		}
	}
	return devices.String(), nil
}
