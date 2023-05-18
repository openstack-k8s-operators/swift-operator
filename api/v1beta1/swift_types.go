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

// SwiftSpec defines the desired state of Swift
type SwiftSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Required
        // SwiftRing - Spec definition for the Ring service of this Swift deployment
        SwiftRing SwiftRingSpec `json:"swiftRing"`

	// +kubebuilder:validation:Required
        // SwiftStorage - Spec definition for the Storage service of this Swift deployment
        SwiftStorage SwiftStorageSpec `json:"swiftStorage"`

	// +kubebuilder:validation:Required
        // SwiftProxy - Spec definition for the Proxy service of this Swift deployment
        SwiftProxy SwiftProxySpec `json:"swiftProxy"`
}

// SwiftStatus defines the observed state of Swift
type SwiftStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

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
