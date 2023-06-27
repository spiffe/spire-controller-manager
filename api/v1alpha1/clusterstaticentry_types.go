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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterStaticEntrySpec defines the desired state of ClusterStaticEntry
type ClusterStaticEntrySpec struct {
	SPIFFEID      string          `json:"spiffeID"`
	ParentID      string          `json:"parentID"`
	Selectors     []string        `json:"selectors"`
	FederatesWith []string        `json:"federatesWith"`
	X509SVIDTTL   metav1.Duration `json:"x509SVIDTTL"`
	JWTSVIDTTL    metav1.Duration `json:"jwtSVIDTTL"`
	DNSNames      []string        `json:"dnsNames"`
	Hint          string          `json:"hint"`
	Admin         bool            `json:"admin,omitempty"`
	Downstream    bool            `json:"downstream,omitempty"`
}

// ClusterStaticEntryStatus defines the observed state of ClusterStaticEntry
type ClusterStaticEntryStatus struct {
	// If the static entry rendered properly.
	Rendered bool `json:"rendered"`

	// If the static entry was masked by another entry.
	Masked bool `json:"masked"`

	// If the static entry was successfully created/updated.
	Set bool `json:"set"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// ClusterStaticEntry is the Schema for the clusterstaticentries API
type ClusterStaticEntry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterStaticEntrySpec   `json:"spec,omitempty"`
	Status ClusterStaticEntryStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterStaticEntryList contains a list of ClusterStaticEntry
type ClusterStaticEntryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterStaticEntry `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterStaticEntry{}, &ClusterStaticEntryList{})
}
