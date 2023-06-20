/*
Copyright 2021 SPIRE Authors.

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

	"regexp"

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
	Scheme           *runtime.Scheme
	Triggerer        reconciler.Triggerer
	IgnoreNamespaces []regexp.Regexp
}

//+kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterspiffeids,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterspiffeids/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=spire.spiffe.io,resources=clusterspiffeids/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	for _, regex := range r.IgnoreNamespaces {
		if !(regex.FindString(req.Namespace) == "") {
			return ctrl.Result{}, nil
		}
	}

	log.FromContext(ctx).V(1).Info("Triggering reconciliation")
	r.Triggerer.Trigger()

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
