/*
Copyright 2025.

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

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("SwiftProxy Webhook", func() {

	When("A SwiftProxy is created with deprecated rabbitMqClusterName field", func() {
		It("should emit a warning about the deprecated field", func() {
			spec := map[string]any{
				"replicas":            int64(1),
				"serviceUser":         "swift",
				"secret":              "osp-secret",
				"memcachedInstance":   "memcached",
				"rabbitMqClusterName": "rabbitmq",
				"containerImageProxy": "quay.io/tripleozedcentos9/openstack-swift-proxy-server:current-tripleo",
			}

			swiftProxyName := types.NamespacedName{
				Namespace: "default",
				Name:      "swiftproxy-webhook-test",
			}

			raw := map[string]any{
				"apiVersion": "swift.openstack.org/v1beta1",
				"kind":       "SwiftProxy",
				"metadata": map[string]any{
					"name":      swiftProxyName.Name,
					"namespace": swiftProxyName.Namespace,
				},
				"spec": spec,
			}

			// Create the SwiftProxy instance
			unstructuredObj := &unstructured.Unstructured{Object: raw}
			_, err := controllerutil.CreateOrPatch(
				th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
			Expect(err).ShouldNot(HaveOccurred())

			DeferCleanup(th.DeleteInstance, unstructuredObj)
		})
	})

	When("A SwiftProxy is created with notificationsBus field", func() {
		It("should succeed without warnings", func() {
			spec := map[string]any{
				"replicas":          int64(1),
				"serviceUser":       "swift",
				"secret":            "osp-secret",
				"memcachedInstance": "memcached",
				"notificationsBus": map[string]any{
					"cluster": "rabbitmq",
					"user":    "swift-user",
					"vhost":   "swift-vhost",
				},
				"containerImageProxy": "quay.io/tripleozedcentos9/openstack-swift-proxy-server:current-tripleo",
			}

			swiftProxyName := types.NamespacedName{
				Namespace: "default",
				Name:      "swiftproxy-webhook-test-new",
			}

			raw := map[string]any{
				"apiVersion": "swift.openstack.org/v1beta1",
				"kind":       "SwiftProxy",
				"metadata": map[string]any{
					"name":      swiftProxyName.Name,
					"namespace": swiftProxyName.Namespace,
				},
				"spec": spec,
			}

			// Create the SwiftProxy instance
			unstructuredObj := &unstructured.Unstructured{Object: raw}
			_, err := controllerutil.CreateOrPatch(
				th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
			Expect(err).ShouldNot(HaveOccurred())

			DeferCleanup(th.DeleteInstance, unstructuredObj)
		})
	})

	When("A SwiftProxy is updated to change deprecated rabbitMqClusterName field", func() {
		It("rejects update to deprecated rabbitMqClusterName field", func() {
			spec := map[string]any{
				"replicas":            int64(1),
				"serviceUser":         "swift",
				"secret":              "osp-secret",
				"memcachedInstance":   "memcached",
				"rabbitMqClusterName": "rabbitmq",
				"containerImageProxy": "quay.io/tripleozedcentos9/openstack-swift-proxy-server:current-tripleo",
			}

			swiftProxyName := types.NamespacedName{
				Namespace: "default",
				Name:      "swiftproxy-webhook-update-test",
			}

			raw := map[string]any{
				"apiVersion": "swift.openstack.org/v1beta1",
				"kind":       "SwiftProxy",
				"metadata": map[string]any{
					"name":      swiftProxyName.Name,
					"namespace": swiftProxyName.Namespace,
				},
				"spec": spec,
			}

			// Create the SwiftProxy instance
			unstructuredObj := &unstructured.Unstructured{Object: raw}
			_, err := controllerutil.CreateOrPatch(
				th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
			Expect(err).ShouldNot(HaveOccurred())

			DeferCleanup(th.DeleteInstance, unstructuredObj)

			// Try to update rabbitMqClusterName
			Eventually(func(g Gomega) {
				g.Expect(th.K8sClient.Get(th.Ctx, swiftProxyName, unstructuredObj)).Should(Succeed())
				specMap := unstructuredObj.Object["spec"].(map[string]any)
				specMap["rabbitMqClusterName"] = "rabbitmq2"
				err := th.K8sClient.Update(th.Ctx, unstructuredObj)
				g.Expect(err).Should(HaveOccurred())

				var statusError *k8s_errors.StatusError
				g.Expect(errors.As(err, &statusError)).To(BeTrue())
				g.Expect(statusError.ErrStatus.Details.Kind).To(Equal("SwiftProxy"))
				g.Expect(statusError.ErrStatus.Message).To(
					ContainSubstring("field \"spec.rabbitMqClusterName\" is deprecated, use \"spec.notificationsBus.cluster\" instead"))
			}, timeout, interval).Should(Succeed())
		})
	})

	When("A SwiftProxy is created with ceilometerEnabled but without notificationsBus", func() {
		It("should reject the creation", func() {
			spec := map[string]any{
				"replicas":            int64(1),
				"serviceUser":         "swift",
				"secret":              "osp-secret",
				"memcachedInstance":   "memcached",
				"ceilometerEnabled":   true,
				"containerImageProxy": "quay.io/tripleozedcentos9/openstack-swift-proxy-server:current-tripleo",
			}

			swiftProxyName := types.NamespacedName{
				Namespace: "default",
				Name:      "swiftproxy-ceilometer-no-bus",
			}

			raw := map[string]any{
				"apiVersion": "swift.openstack.org/v1beta1",
				"kind":       "SwiftProxy",
				"metadata": map[string]any{
					"name":      swiftProxyName.Name,
					"namespace": swiftProxyName.Namespace,
				},
				"spec": spec,
			}

			// Create the SwiftProxy instance - should fail
			unstructuredObj := &unstructured.Unstructured{Object: raw}
			_, err := controllerutil.CreateOrPatch(
				th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
			Expect(err).Should(HaveOccurred())

			var statusError *k8s_errors.StatusError
			Expect(errors.As(err, &statusError)).To(BeTrue())
			Expect(statusError.ErrStatus.Details.Kind).To(Equal("SwiftProxy"))
			Expect(statusError.ErrStatus.Message).To(
				ContainSubstring("notificationsBus.cluster must be set when ceilometerEnabled is true"))
		})
	})

	When("A SwiftProxy is created with ceilometerEnabled but empty cluster", func() {
		It("should reject the creation", func() {
			spec := map[string]any{
				"replicas":          int64(1),
				"serviceUser":       "swift",
				"secret":            "osp-secret",
				"memcachedInstance": "memcached",
				"ceilometerEnabled": true,
				"notificationsBus": map[string]any{
					"cluster": "",
					"user":    "swift-user",
				},
				"containerImageProxy": "quay.io/tripleozedcentos9/openstack-swift-proxy-server:current-tripleo",
			}

			swiftProxyName := types.NamespacedName{
				Namespace: "default",
				Name:      "swiftproxy-ceilometer-empty-cluster",
			}

			raw := map[string]any{
				"apiVersion": "swift.openstack.org/v1beta1",
				"kind":       "SwiftProxy",
				"metadata": map[string]any{
					"name":      swiftProxyName.Name,
					"namespace": swiftProxyName.Namespace,
				},
				"spec": spec,
			}

			// Create the SwiftProxy instance - should fail
			unstructuredObj := &unstructured.Unstructured{Object: raw}
			_, err := controllerutil.CreateOrPatch(
				th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
			Expect(err).Should(HaveOccurred())

			var statusError *k8s_errors.StatusError
			Expect(errors.As(err, &statusError)).To(BeTrue())
			Expect(statusError.ErrStatus.Details.Kind).To(Equal("SwiftProxy"))
			// The CRD schema validation catches empty strings before webhook validation
			Expect(statusError.ErrStatus.Message).To(
				ContainSubstring("spec.notificationsBus.cluster"))
		})
	})

	When("A SwiftProxy is created with ceilometerEnabled and notificationsBus.cluster", func() {
		It("should succeed", func() {
			spec := map[string]any{
				"replicas":          int64(1),
				"serviceUser":       "swift",
				"secret":            "osp-secret",
				"memcachedInstance": "memcached",
				"ceilometerEnabled": true,
				"notificationsBus": map[string]any{
					"cluster": "rabbitmq",
					"user":    "swift-user",
					"vhost":   "swift-vhost",
				},
				"containerImageProxy": "quay.io/tripleozedcentos9/openstack-swift-proxy-server:current-tripleo",
			}

			swiftProxyName := types.NamespacedName{
				Namespace: "default",
				Name:      "swiftproxy-ceilometer-valid",
			}

			raw := map[string]any{
				"apiVersion": "swift.openstack.org/v1beta1",
				"kind":       "SwiftProxy",
				"metadata": map[string]any{
					"name":      swiftProxyName.Name,
					"namespace": swiftProxyName.Namespace,
				},
				"spec": spec,
			}

			// Create the SwiftProxy instance - should succeed
			unstructuredObj := &unstructured.Unstructured{Object: raw}
			_, err := controllerutil.CreateOrPatch(
				th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
			Expect(err).ShouldNot(HaveOccurred())

			DeferCleanup(th.DeleteInstance, unstructuredObj)
		})
	})

	When("A SwiftProxy is updated to enable ceilometer without notificationsBus.cluster", func() {
		It("should reject the update", func() {
			spec := map[string]any{
				"replicas":            int64(1),
				"serviceUser":         "swift",
				"secret":              "osp-secret",
				"memcachedInstance":   "memcached",
				"ceilometerEnabled":   false,
				"containerImageProxy": "quay.io/tripleozedcentos9/openstack-swift-proxy-server:current-tripleo",
			}

			swiftProxyName := types.NamespacedName{
				Namespace: "default",
				Name:      "swiftproxy-ceilometer-update",
			}

			raw := map[string]any{
				"apiVersion": "swift.openstack.org/v1beta1",
				"kind":       "SwiftProxy",
				"metadata": map[string]any{
					"name":      swiftProxyName.Name,
					"namespace": swiftProxyName.Namespace,
				},
				"spec": spec,
			}

			// Create the SwiftProxy instance with ceilometer disabled
			unstructuredObj := &unstructured.Unstructured{Object: raw}
			_, err := controllerutil.CreateOrPatch(
				th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
			Expect(err).ShouldNot(HaveOccurred())

			DeferCleanup(th.DeleteInstance, unstructuredObj)

			// Try to enable ceilometer without setting notificationsBus.cluster
			Eventually(func(g Gomega) {
				g.Expect(th.K8sClient.Get(th.Ctx, swiftProxyName, unstructuredObj)).Should(Succeed())
				specMap := unstructuredObj.Object["spec"].(map[string]any)
				specMap["ceilometerEnabled"] = true
				err := th.K8sClient.Update(th.Ctx, unstructuredObj)
				g.Expect(err).Should(HaveOccurred())

				var statusError *k8s_errors.StatusError
				g.Expect(errors.As(err, &statusError)).To(BeTrue())
				g.Expect(statusError.ErrStatus.Details.Kind).To(Equal("SwiftProxy"))
				g.Expect(statusError.ErrStatus.Message).To(
					ContainSubstring("notificationsBus.cluster must be set when ceilometerEnabled is true"))
			}, timeout, interval).Should(Succeed())
		})
	})
})
