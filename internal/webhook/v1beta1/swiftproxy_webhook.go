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

// Package v1beta1 implements webhook handlers for SwiftProxy v1beta1 API resources.
package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
)

// nolint:unused
// log is for logging in this package.
var swiftproxylog = logf.Log.WithName("swiftproxy-resource")

// SetupSwiftProxyWebhookWithManager registers the webhook for SwiftProxy in the manager.
func SetupSwiftProxyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&swiftv1beta1.SwiftProxy{}).
		WithValidator(&SwiftProxyCustomValidator{}).
		WithDefaulter(&SwiftProxyCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-swift-openstack-org-v1beta1-swiftproxy,mutating=true,failurePolicy=fail,sideEffects=None,groups=swift.openstack.org,resources=swiftproxies,verbs=create;update,versions=v1beta1,name=mswiftproxy-v1beta1.kb.io,admissionReviewVersions=v1

// SwiftProxyCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind SwiftProxy when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type SwiftProxyCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &SwiftProxyCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind SwiftProxy.
func (d *SwiftProxyCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	swiftproxy, ok := obj.(*swiftv1beta1.SwiftProxy)

	if !ok {
		return fmt.Errorf("expected a SwiftProxy object but got %T: %w", obj, ErrInvalidObjectType)
	}
	swiftproxylog.Info("Defaulting for SwiftProxy", "name", swiftproxy.GetName())

	// Call the Default method on the SwiftProxy type
	swiftproxy.Default()

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-swift-openstack-org-v1beta1-swiftproxy,mutating=false,failurePolicy=fail,sideEffects=None,groups=swift.openstack.org,resources=swiftproxies,verbs=create;update,versions=v1beta1,name=vswiftproxy-v1beta1.kb.io,admissionReviewVersions=v1

// SwiftProxyCustomValidator struct is responsible for validating the SwiftProxy resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type SwiftProxyCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &SwiftProxyCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type SwiftProxy.
func (v *SwiftProxyCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	swiftproxy, ok := obj.(*swiftv1beta1.SwiftProxy)
	if !ok {
		return nil, fmt.Errorf("expected a SwiftProxy object but got %T: %w", obj, ErrInvalidObjectType)
	}
	swiftproxylog.Info("Validation for SwiftProxy upon creation", "name", swiftproxy.GetName())

	// Call the ValidateCreate method on the SwiftProxy type
	return swiftproxy.ValidateCreate()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type SwiftProxy.
func (v *SwiftProxyCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	swiftproxy, ok := newObj.(*swiftv1beta1.SwiftProxy)
	if !ok {
		return nil, fmt.Errorf("expected a SwiftProxy object for the newObj but got %T: %w", newObj, ErrInvalidObjectType)
	}
	swiftproxylog.Info("Validation for SwiftProxy upon update", "name", swiftproxy.GetName())

	// Call the ValidateUpdate method on the SwiftProxy type
	return swiftproxy.ValidateUpdate(oldObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type SwiftProxy.
func (v *SwiftProxyCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	swiftproxy, ok := obj.(*swiftv1beta1.SwiftProxy)
	if !ok {
		return nil, fmt.Errorf("expected a SwiftProxy object but got %T: %w", obj, ErrInvalidObjectType)
	}
	swiftproxylog.Info("Validation for SwiftProxy upon deletion", "name", swiftproxy.GetName())

	// Call the ValidateDelete method on the SwiftProxy type
	return swiftproxy.ValidateDelete()
}
