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
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RingCreateHash = "ringcreate"
	DeviceListHash = "devicelist"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SwiftRingSpec defines the desired state of SwiftRing
type SwiftRingSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	SwiftRingSpecCore `json:",inline"`

	// +kubebuilder:validation:Required
	// Image URL for Swift proxy service
	ContainerImage string `json:"containerImage"`
}

// SwiftRingSpec defines the desired state of SwiftRing
type SwiftRingSpecCore struct {

	// +kubebuilder:validation:Required
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// Number of Swift data replicas (=copies)
	RingReplicas *int64 `json:"ringReplicas"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	// Partition power of the Swift rings
	PartPower *int64 `json:"partPower"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// Minimum numbeir of hours to restrict moving a partition more than once
	MinPartHours *int64 `json:"minPartHours"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=swift-conf
	// Name of Secret containing swift.conf
	SwiftConfSecret string `json:"swiftConfSecret"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// TLS - Parameters related to the TLS
	TLS tls.Ca `json:"tls,omitempty"`
}

// SwiftRingStatus defines the observed state of SwiftRing
type SwiftRingStatus struct {
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// SwiftRing is the Schema for the swiftrings API
type SwiftRing struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SwiftRingSpec   `json:"spec,omitempty"`
	Status SwiftRingStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SwiftRingList contains a list of SwiftRing
type SwiftRingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SwiftRing `json:"items"`
}

type SwiftDisk struct {
	Device string `json:"device"`
	Path   string `json:"path"`
	Weight int32  `json:"weight"`
	Region int32  `json:"region"`
	Zone   int32  `json:"zone"`
}

func init() {
	SchemeBuilder.Register(&SwiftRing{}, &SwiftRingList{})
}
