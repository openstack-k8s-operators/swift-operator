/*
Copyright 2026.

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

package v1beta1

import (
	"strings"
	"testing"

	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestSwiftNestedServicePortNamesAreValidated(t *testing.T) {
	for _, endpoint := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		t.Run(string(endpoint), func(t *testing.T) {
			wrongName := "wrong"
			override := ProxyOverrideSpec{Service: map[service.Endpoint]service.RoutedOverrideSpec{
				endpoint: {
					OverrideSpec: service.OverrideSpec{Spec: &service.OverrideServiceSpec{
						Ports: []service.OverrideServicePort{{Name: wrongName, Port: 8081}},
					}},
				},
			}}

			check := func(name string, errs field.ErrorList) {
				t.Helper()
				if len(errs) == 0 {
					t.Fatalf("%s accepted invalid port name %q", name, wrongName)
				}
				if !strings.Contains(errs[0].Detail, `must be "`+string(endpoint)+`"`) {
					t.Errorf("%s error = %q, want a %q port-name error", name, errs[0].Detail, endpoint)
				}
			}

			specCore := SwiftSpecCore{SwiftProxy: SwiftProxySpecCore{Override: override}}
			check("SwiftSpecCore create", specCore.ValidateCreate(field.NewPath("spec"), "swift"))
			check("SwiftSpecCore update", specCore.ValidateUpdate(SwiftSpecCore{}, field.NewPath("spec"), "swift"))

			spec := SwiftSpec{SwiftProxy: SwiftProxySpec{SwiftProxySpecCore: SwiftProxySpecCore{Override: override}}}
			check("SwiftSpec create", spec.ValidateCreate(field.NewPath("spec"), "swift"))
			check("SwiftSpec update", spec.ValidateUpdate(SwiftSpec{}, field.NewPath("spec"), "swift"))
		})
	}
}
