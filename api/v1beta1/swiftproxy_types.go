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
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PasswordSelector to identify the AdminUser password from the Secret
type PasswordSelector struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="SwiftPassword"
	// Service - Selector to get the Swift service password from the Secret
	Service string `json:"service"`
}

// SwiftProxySpec defines the desired state of SwiftProxy
type SwiftProxySpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=0
	// Replicas of Swift Proxy
	Replicas *int32 `json:"replicas"`

	// +kubebuilder:validation:Required
	// Swift Proxy Container Image URL
	ContainerImageProxy string `json:"containerImageProxy"`

	// +kubebuilder:default=swift
	// ServiceUser - optional username used for this service to register in Swift
	ServiceUser string `json:"serviceUser"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=osp-secret
	// Secret containing OpenStack password information for Swift service user password
	Secret string `json:"secret"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default={service: SwiftPassword}
	// PasswordSelector - Selector to choose the Swift user password from the Secret
	PasswordSelectors PasswordSelector `json:"passwordSelectors"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=swift-conf
	// Name of Secret containing swift.conf
	SwiftConfSecret string `json:"swiftConfSecret"`

	// +kubebuilder:validation:Optional
	// Override, provides the ability to override the generated manifest of several child resources.
	Override ProxyOverrideSpec `json:"override,omitempty"`

	// +kubebuilder:validation:Optional
	// NetworkAttachments is a list of NetworkAttachment resource names to expose the services to the given network
	NetworkAttachments []string `json:"networkAttachments,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=""
	// List of memcached servers.
	MemcachedServers string `json:"memcachedServers"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// TLS - Parameters related to the TLS
	TLS tls.API `json:"tls,omitempty"`

	// DefaultConfigOverwrite - can be used to add additionalfiles. Those get
	// added to the service config dir in /etc/<servicename>-conf.d
	DefaultConfigOverwrite map[string]string `json:"defaultConfigOverwrite,omitempty"`
}

// ProxyOverrideSpec to override the generated manifest of several child resources.
type ProxyOverrideSpec struct {
	// Override configuration for the Service created to serve traffic to the cluster.
	// The key must be the endpoint type (public, internal)
	Service map[service.Endpoint]service.RoutedOverrideSpec `json:"service,omitempty"`
}

// SwiftProxyStatus defines the observed state of SwiftProxy
type SwiftProxyStatus struct {
	// ReadyCount of SwiftProxy instances
	ReadyCount int32 `json:"readyCount,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// NetworkAttachments status of the deployment pods
	NetworkAttachments map[string][]string `json:"networkAttachments,omitempty"`

	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="NetworkAttachments",type="string",JSONPath=".status.networkAttachments",description="NetworkAttachments"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// SwiftProxy is the Schema for the swiftproxies API
type SwiftProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SwiftProxySpec   `json:"spec,omitempty"`
	Status SwiftProxyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SwiftProxyList contains a list of SwiftProxy
type SwiftProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SwiftProxy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SwiftProxy{}, &SwiftProxyList{})
}
