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
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/pkg/swift"
	"github.com/openstack-k8s-operators/swift-operator/pkg/swiftproxy"
	"github.com/openstack-k8s-operators/swift-operator/pkg/swiftring"
)

//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete

// NetworkPolicy creates a NetworkPolicy for swift storage services
func NetworkPolicy(
	instance *swiftv1beta1.SwiftStorage, storageNetworkRange string) *networkingv1.NetworkPolicy {

	portAccountServer := intstr.FromInt(int(swift.AccountServerPort))
	portContainerServer := intstr.FromInt(int(swift.ContainerServerPort))
	portObjectServer := intstr.FromInt(int(swift.ObjectServerPort))
	portRsync := intstr.FromInt(int(swift.RsyncPort))

	storageLabels := Labels()
	proxyLabels := swiftproxy.Labels()
	ringLabels := swiftring.Labels()

	storagePeers := []networkingv1.NetworkPolicyPeer{
		{
			PodSelector: &metav1.LabelSelector{
				MatchLabels: storageLabels,
			},
		},
	}

	if storageNetworkRange != "" {
		storagePeers = append(storagePeers, networkingv1.NetworkPolicyPeer{
			IPBlock: &networkingv1.IPBlock{
				CIDR: storageNetworkRange,
			},
		})
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "np-" + instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: storageLabels,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port: &portAccountServer,
						},
						{
							Port: &portContainerServer,
						},
						{
							Port: &portObjectServer,
						},
						{
							Port: &portRsync,
						},
					},
					From: storagePeers,
				},
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port: &portAccountServer,
						},
						{
							Port: &portContainerServer,
						},
						{
							Port: &portObjectServer,
						},
					},
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: proxyLabels,
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: ringLabels,
							},
						},
					},
				},
			},
		},
	}
}

// NetworkPolicyStruct provides utilities for managing NetworkPolicy resources
type NetworkPolicyStruct struct {
	networkPolicy *networkingv1.NetworkPolicy
	timeout       time.Duration
}

// NewNetworkPolicy returns an initialized NetworkPolicy.
func NewNetworkPolicy(
	networkPolicy *networkingv1.NetworkPolicy,
	timeout time.Duration,
) *NetworkPolicyStruct {
	return &NetworkPolicyStruct{
		networkPolicy: networkPolicy,
		timeout:       timeout,
	}
}

// CreateOrPatch creates or patches the NetworkPolicy resource
// TODO: add this to lib-common
func (np *NetworkPolicyStruct) CreateOrPatch(
	ctx context.Context,
	h *helper.Helper,
) (ctrl.Result, error) {
	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      np.networkPolicy.Name,
			Namespace: np.networkPolicy.Namespace,
		},
	}

	op, err := controllerutil.CreateOrPatch(ctx, h.GetClient(), networkPolicy, func() error {
		networkPolicy.Spec = np.networkPolicy.Spec
		err := controllerutil.SetControllerReference(h.GetBeforeObject(), networkPolicy, h.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		h.GetLogger().Error(err, "Error creating NetworkPolicy")
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		h.GetLogger().Info(fmt.Sprintf("NetworkPolicy %s - %s", np.networkPolicy.Name, op))
	}

	return ctrl.Result{}, nil
}
