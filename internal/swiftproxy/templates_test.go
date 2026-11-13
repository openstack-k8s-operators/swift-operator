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

package swiftproxy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/clusterdns"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/internal/swift"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSecretTemplatesRenderVHostServerNamesAndAliases(t *testing.T) {
	const namespace = "swiftproxy-test"
	publicURL := "https://swift-public.example.test"
	internalURL := "https://swift-internal.example.test"
	lbOverride := service.OverrideSpec{
		Spec: &service.OverrideServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
	}

	instance := &swiftv1beta1.SwiftProxy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "swiftproxy",
			Namespace: namespace,
		},
	}
	instance.Spec.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{
		service.EndpointPublic:   {OverrideSpec: lbOverride, EndpointURL: &publicURL},
		service.EndpointInternal: {OverrideSpec: lbOverride, EndpointURL: &internalURL},
	}

	rendered := renderHTTPDConfig(t, instance)
	for _, endpoint := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		defaultName := fmt.Sprintf("%s-%s.%s.svc", swift.ServiceName, endpoint, namespace)
		customHost := strings.TrimPrefix(
			map[service.Endpoint]string{
				service.EndpointPublic:   publicURL,
				service.EndpointInternal: internalURL,
			}[endpoint],
			"https://",
		)
		for _, want := range []string{
			"ServerName " + customHost,
			"ServerAlias " + defaultName,
			"ServerAlias " + defaultName + "." + clusterdns.GetDNSClusterDomain(),
		} {
			if !strings.Contains(rendered, want) {
				t.Errorf("rendered httpd config does not contain %q:\n%s", want, rendered)
			}
		}
	}

	// A Route-backed endpointURL (no LoadBalancer) keeps the default ServerName:
	// the router sends no SNI, so the route host must not become a ServerName.
	instance.Spec.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{
		service.EndpointPublic: {EndpointURL: &publicURL},
	}
	rendered = renderHTTPDConfig(t, instance)
	if strings.Contains(rendered, "swift-public.example.test") {
		t.Errorf("route host must not appear in the httpd config:\n%s", rendered)
	}
	if want := fmt.Sprintf("ServerName %s-public.%s.svc", swift.ServiceName, namespace); !strings.Contains(rendered, want) {
		t.Errorf("rendered httpd config does not contain %q:\n%s", want, rendered)
	}

	instance.Spec.Override.Service = nil
	rendered = renderHTTPDConfig(t, instance)
	for _, endpoint := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		defaultName := fmt.Sprintf("%s-%s.%s.svc", swift.ServiceName, endpoint, namespace)
		for _, want := range []string{
			"ServerName " + defaultName,
			"ServerAlias " + defaultName + "." + clusterdns.GetDNSClusterDomain(),
		} {
			if !strings.Contains(rendered, want) {
				t.Errorf("rendered default httpd config does not contain %q:\n%s", want, rendered)
			}
		}
	}
}

func renderHTTPDConfig(t *testing.T, instance *swiftv1beta1.SwiftProxy) string {
	t.Helper()

	templates, err := SecretTemplates(
		instance, nil, "", "", "", &memcachedv1.Memcached{}, "", "", "", "", 60, false, "", "",
	)
	if err != nil {
		t.Fatal(err)
	}

	templatePath := filepath.Join("..", "..", "templates", "swiftproxy", "config", "httpd.conf")
	templateData, err := os.ReadFile(templatePath) //nolint:gosec // G304: fixed in-repo template path
	if err != nil {
		t.Fatal(err)
	}

	rendered, err := util.ExecuteTemplateData(string(templateData), templates[0].ConfigOptions)
	if err != nil {
		t.Fatal(err)
	}

	return rendered
}
