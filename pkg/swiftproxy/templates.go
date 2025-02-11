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

package swiftproxy

import (
	"fmt"

	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	swift "github.com/openstack-k8s-operators/swift-operator/pkg/swift"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
)

// SecretTemplates -
func SecretTemplates(
	instance *swiftv1beta1.SwiftProxy,
	labels map[string]string,
	keystonePublicURL string,
	keystoneInternalURL string,
	password string,
	mc *memcachedv1.Memcached,
	bindIP string,
	secretRef string,
	keystoneRegion string,
	transportURL string,
	httpdOverrideSecret *corev1.Secret,
) ([]util.Template, error) {
	templateParameters := make(map[string]interface{})
	templateParameters["ServiceUser"] = instance.Spec.ServiceUser
	templateParameters["ServicePassword"] = password
	templateParameters["KeystonePublicURL"] = keystonePublicURL
	templateParameters["KeystoneInternalURL"] = keystoneInternalURL
	templateParameters["MemcachedServers"] = mc.GetMemcachedServerListString()
	templateParameters["MemcachedTLS"] = mc.GetMemcachedTLSSupport()
	templateParameters["BindIP"] = bindIP
	templateParameters["SecretRef"] = secretRef
	templateParameters["KeystoneRegion"] = keystoneRegion
	templateParameters["TransportURL"] = transportURL

	// create httpd  vhost template parameters
	customTemplates := map[string]string{}
	httpdVhostConfig := map[string]interface{}{}
	for _, endpt := range []service.Endpoint{service.EndpointInternal, service.EndpointPublic} {
		endptConfig := map[string]interface{}{}
		endptConfig["ServerName"] = fmt.Sprintf("%s-%s.%s.svc", swift.ServiceName, endpt.String(), instance.Namespace)
		endptConfig["TLS"] = false // default TLS to false, and set it bellow to true if enabled
		if instance.Spec.TLS.API.Enabled(endpt) {
			endptConfig["TLS"] = true
			endptConfig["SSLCertificateFile"] = fmt.Sprintf("/etc/pki/tls/certs/%s.crt", endpt.String())
			endptConfig["SSLCertificateKeyFile"] = fmt.Sprintf("/etc/pki/tls/private/%s.key", endpt.String())
		}

		endptConfig["Override"] = false
		if httpdOverrideSecret != nil && len(httpdOverrideSecret.Data) > 0 {
			endptConfig["Override"] = true
			for key, data := range httpdOverrideSecret.Data {
				if len(data) > 0 {
					customTemplates["httpd_custom_"+endpt.String()+"_"+key] = string(data)
				}
			}
		}
		httpdVhostConfig[endpt.String()] = endptConfig
	}
	templateParameters["VHosts"] = httpdVhostConfig
	customData := map[string]string{}
	for key, data := range instance.Spec.DefaultConfigOverwrite {
		customData[key] = data
	}

	// Marshal the templateParameters map to YAML
	yamlData, err := yaml.Marshal(templateParameters)
	if err != nil {
		return []util.Template{}, fmt.Errorf("Error marshalling to YAML: %w", err)
	}
	customData[common.TemplateParameters] = string(yamlData)

	return []util.Template{
		{
			Name:           fmt.Sprintf("%s-config-data", instance.Name),
			Namespace:      instance.Namespace,
			Type:           util.TemplateTypeConfig,
			InstanceType:   instance.Kind,
			ConfigOptions:  templateParameters,
			Labels:         labels,
			CustomData:     customData,
			StringTemplate: customTemplates,
		},
	}, nil
}
