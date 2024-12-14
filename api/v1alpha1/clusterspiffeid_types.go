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

// ClusterSPIFFEIDSpec defines the desired state of ClusterSPIFFEID
type ClusterSPIFFEIDSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// SPIFFEID is the SPIFFE ID template. The node and pod spec are made
	// available to the template under .NodeSpec, .PodSpec respectively.
	SPIFFEIDTemplate string `json:"spiffeIDTemplate"`

	// TTL indicates an upper-bound time-to-live for X509 SVIDs minted for this
	// ClusterSPIFFEID. If unset, a default will be chosen.
	TTL metav1.Duration `json:"ttl,omitempty"`

	// JWTTTL indicates an upper-bound time-to-live for JWT SVIDs minted for this
	// ClusterSPIFFEID.
	JWTTTL metav1.Duration `json:"jwtTtl,omitempty"`

	// DNSNameTemplate represents templates for extra DNS names that are
	// applicable to SVIDs minted for this ClusterSPIFFEID.
	// The node and pod spec are made available to the template under
	// .NodeSpec, .PodSpec respectively.
	DNSNameTemplates []string `json:"dnsNameTemplates,omitempty"`

	// WorkloadSelectorTemplates are templates to produce arbitrary workload
	// selectors that apply to a given workload before it will receive this
	// SPIFFE ID. The rendered value is interpreted by SPIRE and are of the
	// form type:value, where the value may, and often does, contain
	// semicolons, .e.g., k8s:container-image:docker/hello-world
	// The node and pod spec are made available to the template under
	// .NodeSpec, .PodSpec respectively.
	WorkloadSelectorTemplates []string `json:"workloadSelectorTemplates,omitempty"`

	// FederatesWith is a list of trust domain names that workloads that
	// obtain this SPIFFE ID will federate with.
	FederatesWith []string `json:"federatesWith,omitempty"`

	// NamespaceSelector selects the namespaces that are targeted by this
	// CRD.
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// PodSelector selects the pods that are targeted by this
	// CRD.
	PodSelector *metav1.LabelSelector `json:"podSelector,omitempty"`

	// Admin indicates whether or not the SVID can be used to access the SPIRE
	// administrative APIs. Extra care should be taken to only apply this
	// SPIFFE ID to admin workloads.
	Admin bool `json:"admin,omitempty"`

	// Downstream indicates that the entry describes a downstream SPIRE server.
	Downstream bool `json:"downstream,omitempty"`

	// AutoPopulateDNSNames indicates whether or not to auto populate service DNS names.
	AutoPopulateDNSNames bool `json:"autoPopulateDNSNames,omitempty"`

	// Set which Controller Class will act on this object
	// +kubebuilder:validation:Optional
	ClassName string `json:"className,omitempty"`

	// Apply this ID only if there are no other matching non fallback ClusterSPIFFEIDs.
	// +kubebuilder:validation:Optional
	Fallback bool `json:"fallback,omitempty"`

	// Set the entry hint
	// +kubebuilder:validation:Optional
	Hint string `json:"hint,omitempty"`
}

// ClusterSPIFFEIDStatus defines the observed state of ClusterSPIFFEID
type ClusterSPIFFEIDStatus struct {
	// Stats produced by the last entry reconciliation run
	// +kubebuilder:validation:Optional
	Stats ClusterSPIFFEIDStats `json:"stats"`
}

// ClusterSPIFFEIDStats contain entry reconciliation statistics.
type ClusterSPIFFEIDStats struct {
	// How many namespaces were selected.
	// +kubebuilder:validation:Optional
	NamespacesSelected int `json:"namespacesSelected"`

	// How many (selected) namespaces were ignored (based on configuration).
	// +kubebuilder:validation:Optional
	NamespacesIgnored int `json:"namespacesIgnored"`

	// How many pods were selected out of the namespaces.
	// +kubebuilder:validation:Optional
	PodsSelected int `json:"podsSelected"`

	// How many failures were encountered rendering an entry selected pods.
	// This could be due to either a bad template in the ClusterSPIFFEID or
	// Pod metadata that when applied to the template did not produce valid
	// entry values.
	// +kubebuilder:validation:Optional
	PodEntryRenderFailures int `json:"podEntryRenderFailures"`

	// How many entries were masked by entries for other ClusterSPIFFEIDs.
	// This happens when one or more ClusterSPIFFEIDs produce an entry for
	// the same pod with the same set of workload selectors.
	// +kubebuilder:validation:Optional
	EntriesMasked int `json:"entriesMasked"`

	// How many entries are to be set for this ClusterSPIFFEID. In nominal
	// conditions, this should reflect the number of pods selected, but not
	// always if there were problems encountered rendering an entry for the pod
	// (RenderFailures) or entries are masked (EntriesMasked).
	// +kubebuilder:validation:Optional
	EntriesToSet int `json:"entriesToSet"`

	// How many entries were unable to be set due to failures to create or
	// update the entries via the SPIRE Server API.
	// +kubebuilder:validation:Optional
	EntryFailures int `json:"entryFailures"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// ClusterSPIFFEID is the Schema for the clusterspiffeids API
type ClusterSPIFFEID struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterSPIFFEIDSpec `json:"spec,omitempty"`
	// +optional
	Status ClusterSPIFFEIDStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterSPIFFEIDList contains a list of ClusterSPIFFEID
type ClusterSPIFFEIDList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterSPIFFEID `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterSPIFFEID{}, &ClusterSPIFFEIDList{})
}
