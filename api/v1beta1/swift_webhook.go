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
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// SwiftDefaults -
type SwiftDefaults struct {
	AccountContainerImageURL	string
	ContainerContainerImageURL	string
	ObjectContainerImageURL		string
	ProxyContainerImageURL		string
	MemcachedContainerImageURL	string
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

	if spec.SwiftStorage.ContainerImageMemcached == "" {
		spec.SwiftStorage.ContainerImageMemcached = swiftDefaults.MemcachedContainerImageURL
	}

	// proxy
	if spec.SwiftProxy.ContainerImageProxy == "" {
		spec.SwiftProxy.ContainerImageProxy = swiftDefaults.ProxyContainerImageURL
	}

	if spec.SwiftProxy.ContainerImageMemcached == "" {
		spec.SwiftProxy.ContainerImageMemcached = swiftDefaults.MemcachedContainerImageURL
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-swift-openstack-org-v1beta1-swift,mutating=false,failurePolicy=fail,sideEffects=None,groups=swift.openstack.org,resources=swifts,verbs=create;update,versions=v1beta1,name=vswift.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Swift{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Swift) ValidateCreate() error {
	swiftlog.Info("validate create", "name", r.Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Swift) ValidateUpdate(old runtime.Object) error {
	swiftlog.Info("validate update", "name", r.Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Swift) ValidateDelete() error {
	swiftlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
