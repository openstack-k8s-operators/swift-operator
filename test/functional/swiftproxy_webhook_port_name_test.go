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

package functional_test

import (
	"errors"
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports

	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
)

var _ = DescribeTable("SwiftProxy Service port name validation", func(endpoint service.Endpoint) {
	namespaceName := fmt.Sprintf("swiftproxy-port-%s-%d", endpoint, GinkgoRandomSeed())
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	Expect(k8sClient.Create(th.Ctx, namespace)).To(Succeed())
	DeferCleanup(func() {
		Expect(k8sClient.Delete(th.Ctx, namespace)).To(Succeed())
	})

	validName := string(endpoint)
	proxyName := "swiftproxy-port-name"
	proxy := newSwiftProxyPortOverride(namespaceName, proxyName, endpoint, validName)
	_, err := controllerutil.CreateOrPatch(th.Ctx, k8sClient, proxy, func() error { return nil })
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(th.DeleteInstance, proxy)

	// A correct name remains valid on update.
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(th.Ctx, types.NamespacedName{Name: proxyName, Namespace: namespaceName}, proxy)).To(Succeed())
		spec := proxy.Object["spec"].(map[string]any)
		spec["apiTimeout"] = int64(61)
		g.Expect(k8sClient.Update(th.Ctx, proxy)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	// A wrong name is rejected on create.
	wrongCreate := newSwiftProxyPortOverride(namespaceName, "swiftproxy-port-name-create-invalid", endpoint, "wrong")
	_, err = controllerutil.CreateOrPatch(th.Ctx, k8sClient, wrongCreate, func() error { return nil })
	expectPortNameValidationError(err, validName)

	// A wrong name is also rejected on update.
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(th.Ctx, types.NamespacedName{Name: proxyName, Namespace: namespaceName}, proxy)).To(Succeed())
		spec := proxy.Object["spec"].(map[string]any)
		override := spec["override"].(map[string]any)
		services := override["service"].(map[string]any)
		endpointOverride := services[string(endpoint)].(map[string]any)
		serviceSpec := endpointOverride["spec"].(map[string]any)
		ports := serviceSpec["ports"].([]any)
		ports[0].(map[string]any)["name"] = "wrong"

		err := k8sClient.Update(th.Ctx, proxy)
		var statusError *k8s_errors.StatusError
		g.Expect(errors.As(err, &statusError)).To(BeTrue())
		g.Expect(statusError.ErrStatus.Message).To(ContainSubstring(fmt.Sprintf("must be %q", validName)))
	}, timeout, interval).Should(Succeed())
},
	Entry("public", service.EndpointPublic),
	Entry("internal", service.EndpointInternal),
)

func newSwiftProxyPortOverride(namespace, name string, endpoint service.Endpoint, portName string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "swift.openstack.org/v1beta1",
		"kind":       "SwiftProxy",
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]any{
			"replicas":            int64(1),
			"serviceUser":         "swift",
			"secret":              "osp-secret",
			"memcachedInstance":   "memcached",
			"apiTimeout":          int64(60),
			"containerImageProxy": "quay.io/tripleozedcentos9/openstack-swift-proxy-server:current-tripleo",
			"override": map[string]any{
				"service": map[string]any{
					string(endpoint): map[string]any{
						"spec": map[string]any{
							"ports": []any{
								map[string]any{
									"name": portName,
									"port": int64(8080),
								},
							},
						},
					},
				},
			},
		},
	}}
}

func expectPortNameValidationError(err error, validName string) {
	Expect(err).To(HaveOccurred())
	var statusError *k8s_errors.StatusError
	Expect(errors.As(err, &statusError)).To(BeTrue())
	Expect(statusError.ErrStatus.Message).To(ContainSubstring(fmt.Sprintf("must be %q", validName)))
}

var urlCaseCounter atomic.Int32

var _ = DescribeTable("SwiftProxy endpointURL validation", func(endpoint service.Endpoint, url string, port int64, wantErr string) {
	namespaceName := fmt.Sprintf("swiftproxy-url-%s-%d", endpoint, urlCaseCounter.Add(1))
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	Expect(k8sClient.Create(th.Ctx, namespace)).To(Succeed())
	DeferCleanup(func() {
		Expect(k8sClient.Delete(th.Ctx, namespace)).To(Succeed())
	})

	proxy := newSwiftProxyPortOverride(namespaceName, "swiftproxy-url", endpoint, string(endpoint))
	override := proxy.Object["spec"].(map[string]any)["override"].(map[string]any)["service"].(map[string]any)[string(endpoint)].(map[string]any)
	override["endpointURL"] = url
	if port == 0 {
		delete(override, "spec")
	} else {
		override["spec"].(map[string]any)["ports"].([]any)[0].(map[string]any)["port"] = port
	}
	_, err := controllerutil.CreateOrPatch(th.Ctx, k8sClient, proxy, func() error { return nil })
	if wantErr == "" {
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(th.DeleteInstance, proxy)
		return
	}
	Expect(err).To(HaveOccurred())
	var statusError *k8s_errors.StatusError
	Expect(errors.As(err, &statusError)).To(BeTrue())
	Expect(statusError.ErrStatus.Message).To(ContainSubstring(wantErr))
},
	Entry("public FQDN", service.EndpointPublic, "https://swift.example.com", int64(0), ""),
	Entry("public FQDN with matching port", service.EndpointPublic, "https://swift.example.com:8443", int64(8443), ""),
	Entry("public bare short name", service.EndpointPublic, "https://swift", int64(0), "endpointURL"),
	Entry("public missing scheme", service.EndpointPublic, "swift.example.com", int64(0), "endpointURL"),
	Entry("public IP literal", service.EndpointPublic, "https://192.0.2.10", int64(0), "endpointURL"),
	Entry("public port mismatch", service.EndpointPublic, "https://swift.example.com:8443", int64(9999), "endpointURL"),
	Entry("internal FQDN", service.EndpointInternal, "https://swift-int.example.com", int64(0), ""),
	Entry("internal bare short name", service.EndpointInternal, "https://swift", int64(0), "endpointURL"),
	Entry("internal missing scheme", service.EndpointInternal, "swift-int.example.com", int64(0), "endpointURL"),
	Entry("internal IP literal", service.EndpointInternal, "https://192.0.2.10", int64(0), "endpointURL"),
	Entry("internal port mismatch", service.EndpointInternal, "https://swift-int.example.com:8443", int64(9999), "endpointURL"),
)
