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
	common_webhook "github.com/openstack-k8s-operators/lib-common/modules/common/webhook"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SwiftProxyDefaults -
type SwiftProxyDefaults struct {
	APITimeout int
}

var swiftProxyDefaults SwiftProxyDefaults

// log is for logging in this package.
var swiftproxylog = logf.Log.WithName("swiftproxy-resource")

func SetupSwiftProxyDefaults(defaults SwiftProxyDefaults) {
	swiftProxyDefaults = defaults
	swiftproxylog.Info("SwiftProxy defaults initialized", "defaults", defaults)
}

var _ webhook.Defaulter = &SwiftProxy{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *SwiftProxy) Default() {
	swiftproxylog.Info("default", "name", r.Name)

	r.Spec.Default()
}

// Default - set defaults for this SwiftProxy spec
func (spec *SwiftProxySpec) Default() {
	spec.SwiftProxySpecCore.Default()
}

// Default - set defaults for this SwiftProxySpecCore
func (spec *SwiftProxySpecCore) Default() {
	// NotificationsBus.Cluster is not defaulted - it must be explicitly set if NotificationsBus is configured
	// Migration from deprecated fields is handled by openstack-operator
	// This ensures users make a conscious choice about which cluster to use for notifications
}

var _ webhook.Validator = &SwiftProxy{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *SwiftProxy) ValidateCreate() (admission.Warnings, error) {
	swiftproxylog.Info("validate create", "name", r.Name)

	var allWarns []string
	allErrs := field.ErrorList{}
	basePath := field.NewPath("spec")

	warns, errs := r.Spec.ValidateCreate(basePath, r.Namespace)
	allWarns = append(allWarns, warns...)
	allErrs = append(allErrs, errs...)

	if len(allErrs) != 0 {
		return allWarns, apierrors.NewInvalid(GroupVersion.WithKind("SwiftProxy").GroupKind(), r.Name, allErrs)
	}

	return allWarns, nil
}

// ValidateCreate - Exported function wrapping non-exported validate functions,
// this function can be called externally to validate a SwiftProxy spec.
func (spec *SwiftProxySpec) ValidateCreate(basePath *field.Path, namespace string) ([]string, field.ErrorList) {
	return spec.SwiftProxySpecCore.ValidateCreate(basePath, namespace)
}

// ValidateCreate validates the SwiftProxySpecCore spec during creation
func (spec *SwiftProxySpecCore) ValidateCreate(basePath *field.Path, namespace string) ([]string, field.ErrorList) {
	var allErrs field.ErrorList
	var allWarns []string

	// Validate deprecated fields using reflection-based validation
	warnings, errs := spec.validateDeprecatedFieldsCreate(basePath)
	allWarns = append(allWarns, warnings...)
	allErrs = append(allErrs, errs...)

	// validate the service override key is valid
	allErrs = append(allErrs, service.ValidateRoutedOverrides(basePath.Child("override").Child("service"), spec.Override.Service)...)

	// validate TopologyRef namespace
	allErrs = append(allErrs, spec.ValidateTopology(basePath, namespace)...)

	// validate that if CeilometerEnabled is true, notificationsBus.cluster must be set
	if spec.CeilometerEnabled {
		if spec.NotificationsBus == nil || spec.NotificationsBus.Cluster == "" {
			allErrs = append(allErrs, field.Required(
				basePath.Child("notificationsBus").Child("cluster"),
				"notificationsBus.cluster must be set when ceilometerEnabled is true"))
		}
	}

	return allWarns, allErrs
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *SwiftProxy) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	swiftproxylog.Info("validate update", "name", r.Name)

	oldSwiftProxy, ok := old.(*SwiftProxy)
	if !ok || oldSwiftProxy == nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("unable to convert existing object"))
	}

	var allWarns []string
	allErrs := field.ErrorList{}
	basePath := field.NewPath("spec")

	warns, errs := r.Spec.ValidateUpdate(oldSwiftProxy.Spec, basePath, r.Namespace)
	allWarns = append(allWarns, warns...)
	allErrs = append(allErrs, errs...)

	if len(allErrs) != 0 {
		return allWarns, apierrors.NewInvalid(GroupVersion.WithKind("SwiftProxy").GroupKind(), r.Name, allErrs)
	}

	return allWarns, nil
}

// ValidateUpdate - Exported function wrapping non-exported validate functions,
// this function can be called externally to validate a SwiftProxy spec.
func (spec *SwiftProxySpec) ValidateUpdate(old SwiftProxySpec, basePath *field.Path, namespace string) ([]string, field.ErrorList) {
	return spec.SwiftProxySpecCore.ValidateUpdate(old.SwiftProxySpecCore, basePath, namespace)
}

// ValidateUpdate validates the SwiftProxySpecCore spec during update
func (spec *SwiftProxySpecCore) ValidateUpdate(old SwiftProxySpecCore, basePath *field.Path, namespace string) ([]string, field.ErrorList) {
	var allErrs field.ErrorList
	var allWarns []string

	// Validate deprecated fields using centralized validation
	warnings, errs := spec.validateDeprecatedFieldsUpdate(old, basePath)
	allWarns = append(allWarns, warnings...)
	allErrs = append(allErrs, errs...)

	// validate the service override key is valid
	allErrs = append(allErrs, service.ValidateRoutedOverrides(basePath.Child("override").Child("service"), spec.Override.Service)...)

	// validate TopologyRef namespace
	allErrs = append(allErrs, spec.ValidateTopology(basePath, namespace)...)

	// validate that if CeilometerEnabled is true, notificationsBus.cluster must be set
	if spec.CeilometerEnabled {
		if spec.NotificationsBus == nil || spec.NotificationsBus.Cluster == "" {
			allErrs = append(allErrs, field.Required(
				basePath.Child("notificationsBus").Child("cluster"),
				"notificationsBus.cluster must be set when ceilometerEnabled is true"))
		}
	}

	return allWarns, allErrs
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *SwiftProxy) ValidateDelete() (admission.Warnings, error) {
	swiftproxylog.Info("validate delete", "name", r.Name)

	return nil, nil
}

// getDeprecatedFields returns the centralized list of deprecated fields for SwiftProxySpecCore
func (spec *SwiftProxySpecCore) getDeprecatedFields(old *SwiftProxySpecCore) []common_webhook.DeprecatedFieldUpdate {
	// Handle NewValue pointer - NotificationsBus can be nil
	var newValue *string
	if spec.NotificationsBus != nil {
		newValue = &spec.NotificationsBus.Cluster
	}

	deprecatedFields := []common_webhook.DeprecatedFieldUpdate{
		{
			DeprecatedFieldName: "rabbitMqClusterName",
			NewFieldPath:        []string{"notificationsBus", "cluster"},
			NewDeprecatedValue:  &spec.RabbitMqClusterName,
			NewValue:            newValue,
		},
	}

	// If old spec is provided (UPDATE operation), add old values
	if old != nil {
		deprecatedFields[0].OldDeprecatedValue = &old.RabbitMqClusterName
	}

	return deprecatedFields
}

// validateDeprecatedFieldsCreate validates deprecated fields during CREATE operations
func (spec *SwiftProxySpecCore) validateDeprecatedFieldsCreate(basePath *field.Path) ([]string, field.ErrorList) {
	// Get deprecated fields list (without old values for CREATE)
	deprecatedFieldsUpdate := spec.getDeprecatedFields(nil)

	// Convert to DeprecatedField list for CREATE validation
	deprecatedFields := make([]common_webhook.DeprecatedField, len(deprecatedFieldsUpdate))
	for i, df := range deprecatedFieldsUpdate {
		deprecatedFields[i] = common_webhook.DeprecatedField{
			DeprecatedFieldName: df.DeprecatedFieldName,
			NewFieldPath:        df.NewFieldPath,
			DeprecatedValue:     df.NewDeprecatedValue,
			NewValue:            df.NewValue,
		}
	}

	return common_webhook.ValidateDeprecatedFieldsCreate(deprecatedFields, basePath), nil
}

// validateDeprecatedFieldsUpdate validates deprecated fields during UPDATE operations
func (spec *SwiftProxySpecCore) validateDeprecatedFieldsUpdate(old SwiftProxySpecCore, basePath *field.Path) ([]string, field.ErrorList) {
	// Get deprecated fields list with old values
	deprecatedFields := spec.getDeprecatedFields(&old)
	return common_webhook.ValidateDeprecatedFieldsUpdate(deprecatedFields, basePath)
}
