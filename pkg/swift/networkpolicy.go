/*
Copyright 2022.

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

package swift

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type NetworkPolicy struct {
	networkPolicy *networkingv1.NetworkPolicy
	timeout       int
}

// NewNetworkPolicy returns an initialized NetworkPolicy.
func NewNetworkPolicy(
	networkPolicy *networkingv1.NetworkPolicy,
	labels map[string]string,
	timeout int,
) *NetworkPolicy {
	return &NetworkPolicy{
		networkPolicy: networkPolicy,
		timeout:       timeout,
	}
}

func (np *NetworkPolicy) CreateOrPatch(
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
