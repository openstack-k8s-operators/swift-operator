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

package swift

import (
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
)

// SecretTemplates creates secret templates for swift configuration
func SecretTemplates(instance *swiftv1beta1.Swift, serviceLabels map[string]string) []util.Template {
	templateParameters := make(map[string]any)
	templateParameters["SwiftHashPathPrefix"] = RandomString(16)
	templateParameters["SwiftHashPathSuffix"] = RandomString(16)

	return []util.Template{
		{
			Name:          SwiftConfSecretName,
			Namespace:     instance.Namespace,
			Type:          util.TemplateTypeConfig,
			InstanceType:  instance.Kind,
			ConfigOptions: templateParameters,
			Labels:        serviceLabels,
		},
	}
}
