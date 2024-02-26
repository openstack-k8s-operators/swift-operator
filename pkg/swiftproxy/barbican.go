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

package swiftproxy

import (
	"context"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/openstack"

	swiftv1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"

	gophercloud "github.com/gophercloud/gophercloud"
	gopherstack "github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/keymanager/v1/orders"
	"github.com/gophercloud/gophercloud/openstack/keymanager/v1/secrets"
)

func GetBarbicanSecret(instance *swiftv1.SwiftProxy, h *helper.Helper, keystonePublicURL string, password string) (string, error) {
	secretRef := ""

	caCerts := []string{}
	if len(instance.Spec.TLS.CaBundleSecretName) > 0 {
		cacertSecret, _, err := secret.GetSecret(context.TODO(), h, instance.Spec.TLS.CaBundleSecretName, instance.Namespace)
		if err != nil {
			return secretRef, err
		}
		for _, cert := range cacertSecret.Data {
			caCerts = append(caCerts, string(cert))
		}
	}

	authOpts := openstack.AuthOpts{
		AuthURL:    keystonePublicURL,
		Username:   instance.Spec.ServiceUser,
		Password:   password,
		DomainName: "Default",
		TenantName: "service",
		TLS: &openstack.TLSConfig{
			CACerts: caCerts,
		},
	}

	providerClient, err := openstack.GetOpenStackProvider(authOpts)
	if err != nil {
		return secretRef, err
	}

	keyManagerClient, err := gopherstack.NewKeyManagerV1(providerClient, gophercloud.EndpointOpts{})
	if err != nil {
		return secretRef, err
	}

	// Find secret for Swift
	listOpts := secrets.ListOpts{Name: BarbicanSecretName}
	allPages, err := secrets.List(keyManagerClient, listOpts).AllPages()
	if err != nil {
		return secretRef, err
	}
	allSecrets, err := secrets.ExtractSecrets(allPages)
	if err != nil {
		return secretRef, err
	}
	for _, secret := range allSecrets {
		if secret.Status == "ACTIVE" {
			secretRef = secret.SecretRef
		}
	}

	// No secret found so far, create a new one
	if secretRef == "" {
		createOpts := orders.CreateOpts{
			Type: orders.KeyOrder,
			Meta: orders.MetaOpts{
				Name:      BarbicanSecretName,
				Algorithm: "aes",
				BitLength: 256,
				Mode:      "ctr",
			},
		}
		_, err := orders.Create(keyManagerClient, createOpts).Extract()
		if err != nil {
			return secretRef, err
		}

		allPages, err = secrets.List(keyManagerClient, listOpts).AllPages()
		if err != nil {
			return secretRef, err
		}
		allSecrets, err = secrets.ExtractSecrets(allPages)
		if err != nil {
			return secretRef, err
		}
		for _, secret := range allSecrets {
			if secret.Status == "ACTIVE" {
				secretRef = secret.SecretRef
			}
		}
	}
	return secretRef, err
}
