/*
Copyright 2023 SPIRE Authors.

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

package controller

import (
	"context"
	"fmt"
	"regexp"

	"github.com/spiffe/spire-controller-manager/pkg/namespace"
	"github.com/spiffe/spire-controller-manager/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Triggerer            reconciler.Triggerer
	IgnoreNamespaces     []*regexp.Regexp
	AutoPopulateDNSNames bool
}

//+kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterspiffeids,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterspiffeids/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterspiffeids/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	if namespace.IsIgnored(r.IgnoreNamespaces, req.Namespace) {
		return ctrl.Result{}, nil
	}

	log.FromContext(ctx).V(1).Info("Triggering reconciliation")
	r.Triggerer.Trigger()

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// Index endpoints by UID. Later when we reconcile the Pod this will make it easy to find the associated endpoints
	// and auto populate DNS names.
	err := mgr.GetFieldIndexer().IndexField(ctx, &corev1.Endpoints{}, reconciler.EndpointUID, func(rawObj client.Object) []string {
		endpoints, ok := rawObj.(*corev1.Endpoints)
		if !ok {
			log.FromContext(ctx).Error(nil, "unexpected type indexing fields", "type", fmt.Sprintf("%T", rawObj), "expecteed", "*corev1.Endpoints")
			return nil
		}
		var podUIDs []string
		for _, subset := range endpoints.Subsets {
			for _, address := range subset.Addresses {
				if address.TargetRef != nil && address.TargetRef.Kind == "Pod" {
					podUIDs = append(podUIDs, string(address.TargetRef.UID))
				}
			}
			for _, address := range subset.NotReadyAddresses {
				if address.TargetRef != nil && address.TargetRef.Kind == "Pod" {
					podUIDs = append(podUIDs, string(address.TargetRef.UID))
				}
			}
		}

		return podUIDs
	})
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
