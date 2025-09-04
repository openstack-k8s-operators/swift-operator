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
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	swift "github.com/openstack-k8s-operators/swift-operator/pkg/swift"
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
	apiTimeout int,
) []util.Template {
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
	templateParameters["APITimeout"] = apiTimeout

	// MTLS params
	if mc.Status.MTLSCert != "" {
		templateParameters["MemcachedAuthCert"] = fmt.Sprint(memcachedv1.CertMountPath())
		templateParameters["MemcachedAuthKey"] = fmt.Sprint(memcachedv1.KeyMountPath())
		templateParameters["MemcachedAuthCa"] = fmt.Sprint(memcachedv1.CaMountPath())
	} else {
		templateParameters["MemcachedAuthCert"] = ""
		templateParameters["MemcachedAuthKey"] = ""
		templateParameters["MemcachedAuthCa"] = ""
	}

	// create httpd  vhost template parameters
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
		httpdVhostConfig[endpt.String()] = endptConfig
	}
	templateParameters["VHosts"] = httpdVhostConfig
	customData := map[string]string{}
	for key, data := range instance.Spec.DefaultConfigOverwrite {
		customData[key] = data
	}

	return []util.Template{
		{
			Name:          fmt.Sprintf("%s-config-data", instance.Name),
			Namespace:     instance.Namespace,
			Type:          util.TemplateTypeConfig,
			InstanceType:  instance.Kind,
			ConfigOptions: templateParameters,
			Labels:        labels,
			CustomData:    customData,
		},
	}
}
