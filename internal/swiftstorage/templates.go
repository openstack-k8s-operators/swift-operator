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
	"fmt"
	"maps"

	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
)

// ConfigMapTemplates creates ConfigMap templates for swift storage configuration
func ConfigMapTemplates(instance *swiftv1beta1.SwiftStorage,
	labels map[string]string,
	mc *memcachedv1.Memcached,
	bindIP string) []util.Template {
	templateParameters := make(map[string]any)
	templateParameters["MemcachedServers"] = mc.GetMemcachedServerListString()
	templateParameters["MemcachedTLS"] = mc.GetMemcachedTLSSupport()

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

	templateParameters["BindIP"] = bindIP

	customData := map[string]string{}
	maps.Copy(customData, instance.Spec.DefaultConfigOverwrite)

	return []util.Template{
		{
			Name:          fmt.Sprintf("%s-config-data", instance.Name),
			Namespace:     instance.Namespace,
			Type:          util.TemplateTypeConfig,
			InstanceType:  instance.Kind,
			Labels:        labels,
			ConfigOptions: templateParameters,
			CustomData:    customData,
		},
	}
}
