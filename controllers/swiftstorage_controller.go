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

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	service "github.com/openstack-k8s-operators/lib-common/modules/common/service"
	statefulset "github.com/openstack-k8s-operators/lib-common/modules/common/statefulset"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	swiftv1beta1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/swift-operator/pkg/swiftstorage"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/networkattachment"
)

// SwiftStorageReconciler reconciles a SwiftStorage object
type SwiftStorageReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Log     logr.Logger
	Kclient kubernetes.Interface
}

// Partial struct of the NetworkAttachmentDefinition
// config to retrieve the subnet range
type Netconfig struct {
	Name string `json:"name"`
	Ipam struct {
		Range string `json:"range"`
	} `json:"ipam"`
}

//+kubebuilder:rbac:groups=swift.openstack.org,resources=swiftstorages,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=swift.openstack.org,resources=swiftstorages/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=swift.openstack.org,resources=swiftstorages/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch
//+kubebuilder:rbac:groups=memcached.openstack.org,resources=memcacheds,verbs=get;list;watch;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SwiftStorage object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *SwiftStorageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("swiftstorage", req.NamespacedName)

	instance := &swiftv1beta1.SwiftStorage{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			r.Log.Info("SwiftStorage resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.Log.Error(err, "Failed to get SwiftStorage")
		return ctrl.Result{}, err
	}

	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
		cl := condition.CreateList(
			condition.UnknownCondition(condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage),
			condition.UnknownCondition(condition.NetworkAttachmentsReadyCondition, condition.InitReason, condition.NetworkAttachmentsReadyInitMessage),
			condition.UnknownCondition(swiftv1beta1.SwiftStorageReadyCondition, condition.InitReason, condition.ReadyInitMessage),
		)

		instance.Status.Conditions.Init(&cl)

		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	if instance.Status.NetworkAttachments == nil {
		instance.Status.NetworkAttachments = map[string][]string{}
	}

	if !instance.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	helper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		r.Log,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	serviceLabels := swiftstorage.Labels()
	envVars := make(map[string]env.Setter)

	// Create a ConfigMap populated with content from templates/
	tpl := swiftstorage.ConfigMapTemplates(instance, serviceLabels, instance.Spec.MemcachedServers)
	err = configmap.EnsureConfigMaps(ctx, helper, instance, tpl, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Headless Service
	svc, err := service.NewService(swiftstorage.Service(instance), 5*time.Second, nil)
	if err != nil {
		return ctrl.Result{}, err
	}
	ctrlResult, err := svc.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	// networks to attach to
	storageNetworkRange := ""
	for _, netAtt := range instance.Spec.NetworkAttachments {
		nad, err := networkattachment.GetNADWithName(ctx, helper, netAtt, instance.Namespace)
		if err != nil {
			if apierrors.IsNotFound(err) {
				instance.Status.Conditions.Set(condition.FalseCondition(
					condition.NetworkAttachmentsReadyCondition,
					condition.RequestedReason,
					condition.SeverityInfo,
					condition.NetworkAttachmentsReadyWaitingMessage,
					netAtt))
				return ctrl.Result{RequeueAfter: time.Second * 10}, fmt.Errorf("network-attachment-definition %s not found", netAtt)
			}
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.NetworkAttachmentsReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.NetworkAttachmentsReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}

		// Get the storage network subnet range to include it in the
		// NetworkPolicy for the storage pods
		config := Netconfig{}
		if err = json.Unmarshal([]byte(nad.Spec.Config), &config); err != nil {
			return ctrlResult, err
		}
		if config.Name == "storage" {
			storageNetworkRange = config.Ipam.Range
		}
	}

	serviceAnnotations, err := networkattachment.CreateNetworksAnnotation(instance.Namespace, instance.Spec.NetworkAttachments)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed create network annotation from %s: %w",
			instance.Spec.NetworkAttachments, err)
	}

	// Limit internal storage traffic to Swift services
	np := swiftstorage.NewNetworkPolicy(swiftstorage.NetworkPolicy(instance, storageNetworkRange), serviceLabels, 5*time.Second)
	ctrlResult, err = np.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	// Statefulset with all backend containers
	sset := statefulset.NewStatefulSet(swiftstorage.StatefulSet(instance, serviceLabels, serviceAnnotations), 5*time.Second)
	ctrlResult, err = sset.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	instance.Status.ReadyCount = sset.GetStatefulSet().Status.ReadyReplicas
	if instance.Status.ReadyCount == *instance.Spec.Replicas {
		instance.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)
		instance.Status.Conditions.MarkTrue(swiftv1beta1.SwiftStorageReadyCondition, condition.ReadyMessage)
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	r.Log.Info(fmt.Sprintf("Reconciled SwiftStorage '%s' successfully", instance.Name))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SwiftStorageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&swiftv1beta1.SwiftStorage{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Complete(r)
}
