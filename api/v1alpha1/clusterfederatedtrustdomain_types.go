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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterFederatedTrustDomainSpec defines the desired state of ClusterFederatedTrustDomain
type ClusterFederatedTrustDomainSpec struct {
	// TrustDomain is the name of the trust domain to federate with (e.g. example.org)
	// +kubebuilder:validation:Pattern="[a-z0-9._-]{1,255}"
	TrustDomain string `json:"trustDomain"`

	// BundleEndpointURL is the URL of the bundle endpoint. It must be an
	// HTTPS URL and cannot contain userinfo (i.e. username/password).
	BundleEndpointURL string `json:"bundleEndpointURL"`

	// BundleEndpointProfile is the profile for the bundle endpoint.
	BundleEndpointProfile BundleEndpointProfile `json:"bundleEndpointProfile"`

	// TrustDomainBundle is the contents of the bundle for the referenced trust
	// domain. This field is optional when the resource is created.
	// +kubebuilder:validation:Optional
	TrustDomainBundle string `json:"trustDomainBundle,omitempty"`

	// Set which Controller Class will act on this object
	// +kubebuilder:validation:Optional
	ClassName string `json:"className,omitempty"`
}

// BundleEndpointProfile is the profile for the federated trust domain
type BundleEndpointProfile struct {
	// Type is the type of the bundle endpoint profile.
	Type BundleEndpointProfileType `json:"type"`

	// EndpointSPIFFEID is the SPIFFE ID of the bundle endpoint. It is
	// required for the "https_spiffe" profile.
	// +kubebuilder:validation:Optional
	EndpointSPIFFEID string `json:"endpointSPIFFEID,omitempty"`
}

// +kubebuilder:validation:Enum=https_spiffe;https_web
type BundleEndpointProfileType string

const (
	// HTTPSSPIFFEProfileType indicates an "https_spiffe" SPIFFE federation profile
	HTTPSSPIFFEProfileType BundleEndpointProfileType = "https_spiffe"

	// HTTPSWebProfileType indicates an "https_web" SPIFFE federation profile
	HTTPSWebProfileType BundleEndpointProfileType = "https_web"
)

// ClusterFederatedTrustDomainStatus defines the observed state of ClusterFederatedTrustDomain
type ClusterFederatedTrustDomainStatus struct {
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// +kubebuilder:printcolumn:name="Trust Domain",type=string,JSONPath=`.spec.trustDomain`
// +kubebuilder:printcolumn:name="Endpoint URL",type=string,JSONPath=`.spec.bundleEndpointURL`
// ClusterFederatedTrustDomain is the Schema for the clusterfederatedtrustdomains API
type ClusterFederatedTrustDomain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterFederatedTrustDomainSpec   `json:"spec,omitempty"`
	Status ClusterFederatedTrustDomainStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterFederatedTrustDomainList contains a list of ClusterFederatedTrustDomain
type ClusterFederatedTrustDomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterFederatedTrustDomain `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterFederatedTrustDomain{}, &ClusterFederatedTrustDomainList{})
}
