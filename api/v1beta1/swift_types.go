/*
Copyright 2022.

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
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Container image fall-back defaults
	ContainerImageAccount   = "quay.io/podified-antelope-centos9/openstack-swift-account:current-podified"
	ContainerImageContainer = "quay.io/podified-antelope-centos9/openstack-swift-container:current-podified"
	ContainerImageObject    = "quay.io/podified-antelope-centos9/openstack-swift-object:current-podified"
	ContainerImageProxy     = "quay.io/podified-antelope-centos9/openstack-swift-proxy-server:current-podified"
	ContainerImageMemcached = "quay.io/podified-antelope-centos9/openstack-memcached:current-podified"
)

// SwiftSpec defines the desired state of Swift
type SwiftSpec struct {
	// +kubebuilder:validation:Required
	// SwiftRing - Spec definition for the Ring service of this Swift deployment
	SwiftRing SwiftRingSpec `json:"swiftRing"`

	// +kubebuilder:validation:Required
	// SwiftStorage - Spec definition for the Storage service of this Swift deployment
	SwiftStorage SwiftStorageSpec `json:"swiftStorage"`

	// +kubebuilder:validation:Required
	// SwiftProxy - Spec definition for the Proxy service of this Swift deployment
	SwiftProxy SwiftProxySpec `json:"swiftProxy"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=swift-conf
	// Name of Secret containing swift.conf
	SwiftConfSecret string `json:"swiftConfSecret"`

	// Storage class. This is passed to SwiftStorage unless
	// storageClass is explicitly set for the SwiftStorage.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=""
	StorageClass string `json:"storageClass"`

	// +kubebuilder:validation:Optional
	// NetworkAttachments is a list of NetworkAttachment resource names to expose the services to the given network
	NetworkAttachments []string `json:"networkAttachments,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=memcached
	// Memcached instance name.
	MemcachedInstance string `json:"memcachedInstance"`
}

// SwiftStatus defines the observed state of Swift
type SwiftStatus struct {
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="NetworkAttachments",type="string",JSONPath=".status.networkAttachments",description="NetworkAttachments"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// Swift is the Schema for the swifts API
type Swift struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SwiftSpec   `json:"spec,omitempty"`
	Status SwiftStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SwiftList contains a list of Swift
type SwiftList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Swift `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Swift{}, &SwiftList{})
}

// RbacConditionsSet - set the conditions for the rbac object
func (instance Swift) RbacConditionsSet(c *condition.Condition) {
	instance.Status.Conditions.Set(c)
}

// RbacNamespace - return the namespace
func (instance Swift) RbacNamespace() string {
	return instance.Namespace
}

// RbacResourceName - return the name to be used for rbac objects (serviceaccount, role, rolebinding)
func (instance Swift) RbacResourceName() string {
	return "swift-" + instance.Name
}

// IsReady - returns true if Swift is reconciled successfully
func (instance Swift) IsReady() bool {
	return instance.Status.Conditions.IsTrue(condition.ReadyCondition)
}

// SetupDefaults - initializes any CRD field defaults based on environment variables (the defaulting mechanism itself is implemented via webhooks)
func SetupDefaults() {
	// Acquire environmental defaults and initialize Swift defaults with them
	swiftDefaults := SwiftDefaults{
		AccountContainerImageURL:   util.GetEnvVar("RELATED_IMAGE_SWIFT_ACCOUNT_IMAGE_URL_DEFAULT", ContainerImageAccount),
		ContainerContainerImageURL: util.GetEnvVar("RELATED_IMAGE_SWIFT_CONTAINER_IMAGE_URL_DEFAULT", ContainerImageContainer),
		ObjectContainerImageURL:    util.GetEnvVar("RELATED_IMAGE_SWIFT_OBJECT_IMAGE_URL_DEFAULT", ContainerImageObject),
		ProxyContainerImageURL:     util.GetEnvVar("RELATED_IMAGE_SWIFT_PROXY_IMAGE_URL_DEFAULT", ContainerImageProxy),
		MemcachedContainerImageURL: util.GetEnvVar("RELATED_IMAGE_SWIFT_MEMCACHED_IMAGE_URL_DEFAULT", ContainerImageMemcached),
	}

	SetupSwiftDefaults(swiftDefaults)
}
