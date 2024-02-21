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
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
)

func ConfigMapTemplates(instance *swiftv1beta1.SwiftStorage, labels map[string]string, memcachedServers string) []util.Template {
	templateParameters := make(map[string]interface{})
	templateParameters["MemcachedServers"] = memcachedServers

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
			Labels:        labels,
			ConfigOptions: templateParameters,
			CustomData:    customData,
		},
		{
			Name:               fmt.Sprintf("%s-scripts", instance.Name),
			Namespace:          instance.Namespace,
			Type:               util.TemplateTypeScripts,
			InstanceType:       instance.Kind,
			Labels:             labels,
			AdditionalTemplate: map[string]string{"ring-sync.sh": "/common/ring-sync.sh"},
		},
	}
}

func DeviceConfigMapTemplates(instance *swiftv1beta1.SwiftStorage, devices string) []util.Template {
	data := make(map[string]string)
	data["devices.csv"] = devices

	return []util.Template{
		{
			Name:         swiftv1beta1.DeviceConfigMapName,
			Namespace:    instance.Namespace,
			Type:         util.TemplateTypeNone,
			InstanceType: instance.Kind,
			CustomData:   data,
		},
	}
}
