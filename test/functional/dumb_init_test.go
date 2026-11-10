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

package functional_test

import (
	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/internal/swiftproxy"
	"github.com/openstack-k8s-operators/swift-operator/internal/swiftstorage"
)

var _ = Describe("Swift dumb-init commands", func() {
	It("wraps proxy service commands", func() {
		deployment, err := swiftproxy.Deployment(&swiftv1beta1.SwiftProxy{}, nil, nil, "", nil, &memcachedv1.Memcached{})
		Expect(err).NotTo(HaveOccurred())
		expected := map[string][]string{
			"proxy-httpd":  {"--single-child", "--", "/usr/sbin/httpd", "-DFOREGROUND"},
			"proxy-server": {"--single-child", "--", "/usr/bin/swift-proxy-server", "/etc/swift/proxy-server.conf.d", "-v"},
		}
		containers := deployment.Spec.Template.Spec.Containers
		Expect(containers).To(HaveLen(len(expected)))
		for _, container := range containers {
			Expect(expected).To(HaveKey(container.Name))
			Expect(container.Command).To(Equal([]string{"/usr/bin/dumb-init"}), container.Name)
			Expect(container.Args).To(Equal(expected[container.Name]), container.Name)
			delete(expected, container.Name)
		}
	})
	DescribeTable("wraps storage service commands", func(sharderEnabled bool) {
		expected := map[string][]string{
			"account-server":       {"/usr/bin/swift-account-server", "/etc/swift/account-server.conf.d", "-v"},
			"account-replicator":   {"/usr/bin/swift-account-replicator", "/etc/swift/account-server.conf.d", "-v"},
			"account-auditor":      {"/usr/bin/swift-account-auditor", "/etc/swift/account-server.conf.d", "-v"},
			"account-reaper":       {"/usr/bin/swift-account-reaper", "/etc/swift/account-server.conf.d", "-v"},
			"container-server":     {"/usr/bin/swift-container-server", "/etc/swift/container-server.conf.d", "-v"},
			"container-replicator": {"/usr/bin/swift-container-replicator", "/etc/swift/container-server.conf.d", "-v"},
			"container-auditor":    {"/usr/bin/swift-container-auditor", "/etc/swift/container-server.conf.d", "-v"},
			"container-updater":    {"/usr/bin/swift-container-updater", "/etc/swift/container-server.conf.d", "-v"},
			"object-server":        {"/usr/bin/swift-object-server", "/etc/swift/object-server.conf.d", "-v"},
			"object-replicator":    {"/usr/bin/swift-object-replicator", "/etc/swift/object-server.conf.d", "-v"},
			"object-auditor":       {"/usr/bin/swift-object-auditor", "/etc/swift/object-server.conf.d", "-v"},
			"object-updater":       {"/usr/bin/swift-object-updater", "/etc/swift/object-server.conf.d", "-v"},
			"object-expirer":       {"/usr/bin/swift-object-expirer", "/etc/swift/object-expirer.conf.d", "-v"},
			"rsync":                {"/usr/bin/rsync", "--daemon", "--no-detach", "--config=/etc/swift/rsyncd.conf", "--log-file=/dev/stdout"},
			"swift-recon-cron":     {"sh", "-c", "while true; do /usr/bin/swift-recon-cron /etc/swift/object-server.conf.d -v; sleep 300; done"},
			"container-sharder":    {"/usr/bin/swift-container-sharder", "/etc/swift/container-server.conf.d", "-v"},
		}
		if !sharderEnabled {
			delete(expected, "container-sharder")
		}
		instance := &swiftv1beta1.SwiftStorage{}
		instance.Spec.ContainerSharderEnabled = sharderEnabled
		instance.Spec.StorageRequest = "1Gi"
		statefulset, err := swiftstorage.StatefulSet(instance, nil, nil, "", nil, &memcachedv1.Memcached{})
		Expect(err).NotTo(HaveOccurred())
		containers := statefulset.Spec.Template.Spec.Containers
		Expect(containers).To(HaveLen(len(expected)))
		for _, container := range containers {
			Expect(expected).To(HaveKey(container.Name))
			Expect(container.Command).To(Equal([]string{"/usr/bin/dumb-init"}), container.Name)
			Expect(container.Args).To(Equal(append([]string{"--single-child", "--"}, expected[container.Name]...)), container.Name)
			delete(expected, container.Name)
		}
	},
		Entry("without container sharder", false),
		Entry("with container sharder", true),
	)
})
