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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/internal/swift"
)

/*
import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	service "github.com/openstack-k8s-operators/lib-common/modules/common/service"
	statefulset "github.com/openstack-k8s-operators/lib-common/modules/common/statefulset"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/internal/swift"
	"github.com/openstack-k8s-operators/swift-operator/internal/swiftproxy"
	"github.com/openstack-k8s-operators/swift-operator/internal/swiftstorage"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
)
*/

// Service creates a Kubernetes Service for swift storage
func Service(
	instance *swiftv1beta1.SwiftStorage) *corev1.Service {

	storageLabels := Labels()

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
			Labels:    storageLabels,
		},
		Spec: corev1.ServiceSpec{
			Selector: storageLabels,
			Ports: []corev1.ServicePort{
				{
					Name:     "account",
					Port:     swift.AccountServerPort,
					Protocol: corev1.ProtocolTCP,
				},
				{
					Name:     "container",
					Port:     swift.ContainerServerPort,
					Protocol: corev1.ProtocolTCP,
				},
				{
					Name:     "object",
					Port:     swift.ObjectServerPort,
					Protocol: corev1.ProtocolTCP,
				},
				{
					Name:     "rsync",
					Port:     swift.RsyncPort,
					Protocol: corev1.ProtocolTCP,
				},
			},
			ClusterIP: "None", // headless service
		},
	}
}
