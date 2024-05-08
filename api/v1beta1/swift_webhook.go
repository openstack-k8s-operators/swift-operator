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
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SwiftDefaults -
type SwiftDefaults struct {
	AccountContainerImageURL   string
	ContainerContainerImageURL string
	ObjectContainerImageURL    string
	ProxyContainerImageURL     string
}

var swiftDefaults SwiftDefaults

// log is for logging in this package.
var swiftlog = logf.Log.WithName("swift-resource")

func SetupSwiftDefaults(defaults SwiftDefaults) {
	swiftDefaults = defaults
	swiftlog.Info("Swift defaults initialized", "defaults", defaults)
}

func (r *Swift) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-swift-openstack-org-v1beta1-swift,mutating=true,failurePolicy=fail,sideEffects=None,groups=swift.openstack.org,resources=swifts,verbs=create;update,versions=v1beta1,name=mswift.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Swift{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Swift) Default() {
	swiftlog.Info("default", "name", r.Name)

	r.Spec.Default()
}

// Default - set defaults for this Swift spec
func (spec *SwiftSpec) Default() {
	// ring
	if spec.SwiftRing.ContainerImage == "" {
		spec.SwiftRing.ContainerImage = swiftDefaults.ProxyContainerImageURL
	}
	// StorageClass
	if spec.SwiftStorage.StorageClass == "" {
		spec.SwiftStorage.StorageClass = spec.StorageClass
	}

	// storage
	if spec.SwiftStorage.ContainerImageAccount == "" {
		spec.SwiftStorage.ContainerImageAccount = swiftDefaults.AccountContainerImageURL
	}

	if spec.SwiftStorage.ContainerImageContainer == "" {
		spec.SwiftStorage.ContainerImageContainer = swiftDefaults.ContainerContainerImageURL
	}

	if spec.SwiftStorage.ContainerImageObject == "" {
		spec.SwiftStorage.ContainerImageObject = swiftDefaults.ObjectContainerImageURL
	}

	if spec.SwiftStorage.ContainerImageProxy == "" {
		spec.SwiftStorage.ContainerImageProxy = swiftDefaults.ProxyContainerImageURL
	}

	// proxy
	if spec.SwiftProxy.ContainerImageProxy == "" {
		spec.SwiftProxy.ContainerImageProxy = swiftDefaults.ProxyContainerImageURL
	}
}

// Default - set defaults for this Swift core spec (this version is used by OpenStackControlplane webhooks)
func (spec *SwiftSpecCore) Default() {
	// nothing here yet
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-swift-openstack-org-v1beta1-swift,mutating=false,failurePolicy=fail,sideEffects=None,groups=swift.openstack.org,resources=swifts,verbs=create;update,versions=v1beta1,name=vswift.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Swift{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Swift) ValidateCreate() (admission.Warnings, error) {
	swiftlog.Info("validate create", "name", r.Name)

	var allErrs field.ErrorList
	basePath := field.NewPath("spec")
	if err := r.Spec.ValidateCreate(basePath); err != nil {
		allErrs = append(allErrs, err...)
	}

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "swift.openstack.org", Kind: "Swift"},
			r.Name, allErrs)
	}

	return nil, nil
}

// ValidateCreate - Exported function wrapping non-exported validate functions,
// this function can be called externally to validate an swift spec.
func (r *SwiftSpec) ValidateCreate(basePath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// validate the service override key is valid
	allErrs = append(allErrs, service.ValidateRoutedOverrides(
		basePath.Child("swiftProxy").Child("override").Child("service"),
		r.SwiftProxy.Override.Service)...)

	return allErrs
}

func (r *SwiftSpecCore) ValidateCreate(basePath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// validate the service override key is valid
	allErrs = append(allErrs, service.ValidateRoutedOverrides(
		basePath.Child("swiftProxy").Child("override").Child("service"),
		r.SwiftProxy.Override.Service)...)

	return allErrs
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Swift) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	swiftlog.Info("validate update", "name", r.Name)

	oldSwift, ok := old.(*Swift)
	if !ok || oldSwift == nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("unable to convert existing object"))
	}

	var allErrs field.ErrorList
	basePath := field.NewPath("spec")

	if err := r.Spec.ValidateUpdate(oldSwift.Spec, basePath); err != nil {
		allErrs = append(allErrs, err...)
	}

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "swift.openstack.org", Kind: "Swift"},
			r.Name, allErrs)
	}

	return nil, nil
}

// ValidateUpdate - Exported function wrapping non-exported validate functions,
// this function can be called externally to validate an swift spec.
func (r *SwiftSpec) ValidateUpdate(old SwiftSpec, basePath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if *r.SwiftStorage.Replicas < *old.SwiftStorage.Replicas {
		allErrs = append(allErrs, field.Invalid(
			basePath.Child("swiftStorage").Child("replicas"),
			*r.SwiftStorage.Replicas,
			"SwiftStorage does not support scale-in"))
	}

	// validate the service override key is valid
	allErrs = append(allErrs, service.ValidateRoutedOverrides(
		basePath.Child("swiftProxy").Child("override").Child("service"),
		r.SwiftProxy.Override.Service)...)

	return allErrs
}

func (r *SwiftSpecCore) ValidateUpdate(old SwiftSpecCore, basePath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if *r.SwiftStorage.Replicas < *old.SwiftStorage.Replicas {
		allErrs = append(allErrs, field.Invalid(
			basePath.Child("swiftStorage").Child("replicas"),
			*r.SwiftStorage.Replicas,
			"SwiftStorage does not support scale-in"))
	}

	// validate the service override key is valid
	allErrs = append(allErrs, service.ValidateRoutedOverrides(
		basePath.Child("swiftProxy").Child("override").Child("service"),
		r.SwiftProxy.Override.Service)...)

	return allErrs
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Swift) ValidateDelete() (admission.Warnings, error) {
	swiftlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
