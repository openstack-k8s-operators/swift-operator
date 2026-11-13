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
	"fmt"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
)

var _ = Describe("SwiftProxy internal endpoint switched from LoadBalancer to ClusterIP", func() {
	It("drops the dnsmasq hostname annotation", func() {
		namespaceName := fmt.Sprintf("swiftproxy-lb-%d", GinkgoRandomSeed())
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
		Expect(k8sClient.Create(th.Ctx, ns)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(th.Ctx, ns)).To(Succeed()) })

		proxy := newSwiftProxyPortOverride(namespaceName, "swift", service.EndpointInternal, string(service.EndpointInternal))
		internal := proxy.Object["spec"].(map[string]any)["override"].(map[string]any)["service"].(map[string]any)[string(service.EndpointInternal)].(map[string]any)
		internal["spec"].(map[string]any)["type"] = "LoadBalancer"
		_, err := controllerutil.CreateOrPatch(th.Ctx, k8sClient, proxy, func() error { return nil })
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(th.DeleteInstance, proxy)

		svcName := types.NamespacedName{Namespace: namespaceName, Name: "swift-internal"}
		th.SimulateLoadBalancerServiceIP(svcName)
		Eventually(func(g Gomega) {
			g.Expect(th.GetService(svcName).Annotations).To(HaveKey(service.AnnotationHostnameKey))
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(th.Ctx, types.NamespacedName{Namespace: namespaceName, Name: "swift"}, proxy)).To(Succeed())
			svcs := proxy.Object["spec"].(map[string]any)["override"].(map[string]any)["service"].(map[string]any)
			delete(svcs[string(service.EndpointInternal)].(map[string]any), "spec")
			g.Expect(k8sClient.Update(th.Ctx, proxy)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			svc := th.GetService(svcName)
			g.Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			g.Expect(svc.Annotations).NotTo(HaveKey(service.AnnotationHostnameKey))
		}, timeout, interval).Should(Succeed())
	})
})

var _ = Describe("SwiftProxy public endpoint switched from LoadBalancer to ClusterIP", func() {
	It("drops a stale dnsmasq hostname annotation", func() {
		namespaceName := fmt.Sprintf("swiftproxy-pub-%d", GinkgoRandomSeed())
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
		Expect(k8sClient.Create(th.Ctx, ns)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(th.Ctx, ns)).To(Succeed()) })

		proxy := newSwiftProxyPortOverride(namespaceName, "swift", service.EndpointPublic, string(service.EndpointPublic))
		public := proxy.Object["spec"].(map[string]any)["override"].(map[string]any)["service"].(map[string]any)[string(service.EndpointPublic)].(map[string]any)
		public["spec"].(map[string]any)["type"] = "LoadBalancer"
		_, err := controllerutil.CreateOrPatch(th.Ctx, k8sClient, proxy, func() error { return nil })
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(th.DeleteInstance, proxy)

		svcName := types.NamespacedName{Namespace: namespaceName, Name: "swift-public"}
		th.SimulateLoadBalancerServiceIP(svcName)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(th.Ctx, types.NamespacedName{Namespace: namespaceName, Name: "swift"}, proxy)).To(Succeed())
			svcs := proxy.Object["spec"].(map[string]any)["override"].(map[string]any)["service"].(map[string]any)
			delete(svcs[string(service.EndpointPublic)].(map[string]any), "spec")
			g.Expect(k8sClient.Update(th.Ctx, proxy)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(th.GetService(svcName).Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		}, timeout, interval).Should(Succeed())

		// inject the stale registration only now, so a LoadBalancer reconcile
		// cannot clean it up first; the Service update triggers a reconcile
		Eventually(func(g Gomega) {
			svc := th.GetService(svcName)
			svc.Annotations[service.AnnotationHostnameKey] = "swift-public.stale.svc"
			g.Expect(k8sClient.Update(th.Ctx, svc)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			svc := th.GetService(svcName)
			g.Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			g.Expect(svc.Annotations).NotTo(HaveKey(service.AnnotationHostnameKey))
		}, timeout, interval).Should(Succeed())
	})
})
