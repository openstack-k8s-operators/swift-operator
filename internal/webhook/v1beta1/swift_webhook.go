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

// Package v1beta1 implements webhook handlers for Swift v1beta1 API resources.
package v1beta1

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
)

var (
	// ErrInvalidObjectType is returned when an unexpected object type is provided
	ErrInvalidObjectType = errors.New("invalid object type")
)

// nolint:unused
// log is for logging in this package.
var swiftlog = logf.Log.WithName("swift-resource")

// SetupSwiftWebhookWithManager registers the webhook for Swift in the manager.
func SetupSwiftWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&swiftv1beta1.Swift{}).
		WithValidator(&SwiftCustomValidator{}).
		WithDefaulter(&SwiftCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-swift-openstack-org-v1beta1-swift,mutating=true,failurePolicy=fail,sideEffects=None,groups=swift.openstack.org,resources=swifts,verbs=create;update,versions=v1beta1,name=mswift-v1beta1.kb.io,admissionReviewVersions=v1

// SwiftCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind Swift when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type SwiftCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &SwiftCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind Swift.
func (d *SwiftCustomDefaulter) Default(_ context.Context, obj runtime.Object) error {
	swift, ok := obj.(*swiftv1beta1.Swift)

	if !ok {
		return fmt.Errorf("expected an Swift object but got %T: %w", obj, ErrInvalidObjectType)
	}
	swiftlog.Info("Defaulting for Swift", "name", swift.GetName())

	// Call the Default method on the Swift type
	swift.Default()

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-swift-openstack-org-v1beta1-swift,mutating=false,failurePolicy=fail,sideEffects=None,groups=swift.openstack.org,resources=swifts,verbs=create;update,versions=v1beta1,name=vswift-v1beta1.kb.io,admissionReviewVersions=v1

// SwiftCustomValidator struct is responsible for validating the Swift resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type SwiftCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &SwiftCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type Swift.
func (v *SwiftCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	swift, ok := obj.(*swiftv1beta1.Swift)
	if !ok {
		return nil, fmt.Errorf("expected a Swift object but got %T: %w", obj, ErrInvalidObjectType)
	}
	swiftlog.Info("Validation for Swift upon creation", "name", swift.GetName())

	// Call the ValidateCreate method on the Swift type
	return swift.ValidateCreate()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type Swift.
func (v *SwiftCustomValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	swift, ok := newObj.(*swiftv1beta1.Swift)
	if !ok {
		return nil, fmt.Errorf("expected a Swift object for the newObj but got %T: %w", newObj, ErrInvalidObjectType)
	}
	swiftlog.Info("Validation for Swift upon update", "name", swift.GetName())

	// Call the ValidateUpdate method on the Swift type
	return swift.ValidateUpdate(oldObj)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type Swift.
func (v *SwiftCustomValidator) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	swift, ok := obj.(*swiftv1beta1.Swift)
	if !ok {
		return nil, fmt.Errorf("expected a Swift object but got %T: %w", obj, ErrInvalidObjectType)
	}
	swiftlog.Info("Validation for Swift upon deletion", "name", swift.GetName())

	// Call the ValidateDelete method on the Swift type
	return swift.ValidateDelete()
}
