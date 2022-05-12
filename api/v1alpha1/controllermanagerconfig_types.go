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

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cfgv1alpha1 "sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
)

//+kubebuilder:object:root=true

// ControllerManagerConfig is the Schema for the controller manager configuration
type ControllerManagerConfig struct {
	metav1.TypeMeta `json:",inline"`

	// ControllerManagerConfigurationSpec returns the contfigurations for controllers
	cfgv1alpha1.ControllerManagerConfigurationSpec `json:",inline"`

	// ClusterName is the cluster name
	ClusterName string `json:"clusterName"`

	// TrustDomain is the name of the SPIFFE trust domain
	TrustDomain string `json:"trustDomain"`

	// IgnoreNamespaces are the namespaces to ignore
	IgnoreNamespaces []string `json:"ignoreNamespaces"`

	// ValidatingWebhookConfigurationName selects the webhook configuration to manage.
	// Defaults to spire-controller-manager-webhook.
	ValidatingWebhookConfigurationName string `json:"validatingWebhookConfigurationName"`

	// GCInterval is how often SPIRE state is reconciled when the controller
	// is otherwise idle. This impacts how quickly SPIRE state will converge
	// after CRDs are removed or SPIRE state is mutated out from underneath
	// the controller.
	GCInterval time.Duration `json:"gcInterval"`
}

func init() {
	SchemeBuilder.Register(&ControllerManagerConfig{})
}
