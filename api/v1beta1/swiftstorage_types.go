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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SwiftStorageSpec defines the desired state of SwiftStorage
type SwiftStorageSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Replicas int32 `json:"replicas"`

	// +kubebuilder:validation:Required
	// Name of ConfigMap containing Swift rings
	SwiftRingConfigMap string `json:"swiftRingConfigMap,omitempty"`

	// +kubebuilder:validation:Required
	// Name of StorageClass to use for Swift PVs
	StorageClassName string `json:"storageClassName,omitempty"`

	// +kubebuilder:validation:Required
	// Image URL for Swift account service
	ContainerImageAccount string `json:"containerImageAccount"`

	// +kubebuilder:validation:Required
	// Image URL for Swift container service
	ContainerImageContainer string `json:"containerImageContainer"`

	// +kubebuilder:validation:Required
	// Image URL for Swift object service
	ContainerImageObject string `json:"containerImageObject"`

	// +kubebuilder:validation:Required
	// Image URL for Swift proxy service
	ContainerImageProxy string `json:"containerImageProxy"`

	// +kubebuilder:validation:Required
	// Image URL for Memcache servicd
	ContainerImageMemcached string `json:"containerImageMemcached"`
}

// SwiftStorageStatus defines the observed state of SwiftStorage
type SwiftStorageStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// SwiftStorage is the Schema for the swiftstorages API
type SwiftStorage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SwiftStorageSpec   `json:"spec,omitempty"`
	Status SwiftStorageStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SwiftStorageList contains a list of SwiftStorage
type SwiftStorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SwiftStorage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SwiftStorage{}, &SwiftStorageList{})
}
