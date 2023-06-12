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

package swift

import (
	corev1 "k8s.io/api/core/v1"
	"math/rand"
)

func GetSecurityContext() corev1.SecurityContext {
	trueVal := true
	falseVal := false
	user := int64(RunAsUser)

	return corev1.SecurityContext{
		RunAsNonRoot:             &trueVal,
		RunAsUser:                &user,
		AllowPrivilegeEscalation: &falseVal,
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{
				"ALL",
			},
		},
	}
}

func GetLabelsProxy() map[string]string {
	return map[string]string{"app.kubernetes.io/name": "SwiftProxy"}
}

func GetLabelsStorage() map[string]string {
	return map[string]string{"app.kubernetes.io/name": "SwiftStorage"}
}

func GetLabelsRing() map[string]string {
	return map[string]string{"app.kubernetes.io/name": "SwiftRing"}
}

func RandomString(length int) string {
	sample := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	str := make([]byte, length)

	for i := 0; i < length; i++ {
		str[i] = sample[rand.Intn(len(sample))]
	}
	return string(str)
}
