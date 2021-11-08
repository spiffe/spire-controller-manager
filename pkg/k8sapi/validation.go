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

package k8sapi

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/spire-controller-manager/api/v1alpha1"
	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// ValidClusterSPIFFEIDSpec is a parsed and validated ClusterSPIFFEIDSpec
type ValidClusterSPIFFEIDSpec struct {
	SPIFFEIDTemplate          *template.Template
	NamespaceSelector         labels.Selector
	PodSelector               labels.Selector
	TTL                       time.Duration
	FederatesWith             []spiffeid.TrustDomain
	DNSNameTemplates          []*template.Template
	WorkloadSelectorTemplates []*template.Template
	Admin                     bool
}

// ParseClusterSPIFFEIDSpec parses and validates the fields in the ClusterSPIFFEIDSpec
func ParseClusterSPIFFEIDSpec(spec *spirev1alpha1.ClusterSPIFFEIDSpec) (*ValidClusterSPIFFEIDSpec, error) {
	// TODO: update the ClusterSPIFFEID status if it is malformed
	if spec.SPIFFEIDTemplate == "" {
		return nil, errors.New("empty SPIFFEID template")
	}

	spiffeIDTemplate, err := template.New("spiffeIDTemplate").Parse(spec.SPIFFEIDTemplate)
	if err != nil {
		return nil, fmt.Errorf("invalid SPIFFEID template: %w", err)
	}

	var namespaceSelector labels.Selector
	if spec.NamespaceSelector != nil {
		namespaceSelector, err = metav1.LabelSelectorAsSelector(spec.NamespaceSelector)
		if err != nil {
			return nil, err
		}
	}

	var podSelector labels.Selector
	if spec.PodSelector != nil {
		podSelector, err = metav1.LabelSelectorAsSelector(spec.PodSelector)
		if err != nil {
			return nil, err
		}
	}

	federatesWith := make([]spiffeid.TrustDomain, 0, len(spec.FederatesWith))
	for _, value := range spec.FederatesWith {
		td, err := spiffeid.TrustDomainFromString(value)
		if err != nil {
			return nil, fmt.Errorf("invalid federatesWith value: %w", err)
		}
		federatesWith = append(federatesWith, td)
	}

	var dnsNameTemplates []*template.Template
	for _, value := range spec.DNSNameTemplates {
		dnsNameTemplate, err := template.New("dnsNameTemplate").Parse(value)
		if err != nil {
			return nil, fmt.Errorf("invalid dnsNameTemplate value: %w", err)
		}
		dnsNameTemplates = append(dnsNameTemplates, dnsNameTemplate)
	}

	var workloadSelectorTemplates []*template.Template
	for _, value := range spec.WorkloadSelectorTemplates {
		workloadSelectorTemplate, err := template.New("workloadSelectorTemplate").Parse(value)
		if err != nil {
			return nil, fmt.Errorf("invalid workloadSelectorTemplates value: %w", err)
		}
		workloadSelectorTemplates = append(workloadSelectorTemplates, workloadSelectorTemplate)
	}

	return &ValidClusterSPIFFEIDSpec{
		SPIFFEIDTemplate:          spiffeIDTemplate,
		NamespaceSelector:         namespaceSelector,
		PodSelector:               podSelector,
		TTL:                       spec.TTL.Duration,
		FederatesWith:             federatesWith,
		DNSNameTemplates:          dnsNameTemplates,
		WorkloadSelectorTemplates: workloadSelectorTemplates,
		Admin:                     spec.Admin,
	}, nil
}

func ParseClusterFederatedTrustDomainSpec(spec *v1alpha1.ClusterFederatedTrustDomainSpec) (*spireapi.FederationRelationship, error) {
	trustDomain, err := spiffeid.TrustDomainFromString(spec.TrustDomain)
	if err != nil {
		return nil, fmt.Errorf("invalid trustDomain value: %w", err)
	}

	bundleEndpointURL, err := parseBundleEndpointURL(spec.BundleEndpointURL)
	if err != nil {
		return nil, fmt.Errorf("invalid bundleEndpointURL value: %w", err)
	}

	var bundleEndpointProfile spireapi.BundleEndpointProfile
	switch spec.BundleEndpointProfile.Type {
	case v1alpha1.HTTPSWebProfileType:
		if spec.BundleEndpointProfile.EndpointSPIFFEID != "" {
			return nil, fmt.Errorf("invalid endpointSPIFFEID value: not applicable to the %q profile", v1alpha1.HTTPSWebProfileType)
		}
		bundleEndpointProfile = spireapi.HTTPSWebProfile{}
	case v1alpha1.HTTPSSPIFFEProfileType:
		endpointSPIFFEID, err := spiffeid.FromString(spec.BundleEndpointProfile.EndpointSPIFFEID)
		if err != nil {
			return nil, fmt.Errorf("invalid endpointSPIFFEID value: %w", err)
		}
		bundleEndpointProfile = spireapi.HTTPSSPIFFEProfile{
			EndpointSPIFFEID: endpointSPIFFEID,
		}
	default:
		return nil, fmt.Errorf("invalid type value %q", spec.BundleEndpointProfile.Type)
	}

	var trustDomainBundle *spiffebundle.Bundle
	if spec.TrustDomainBundle != "" {
		trustDomainBundle, err = spiffebundle.Read(trustDomain, strings.NewReader(spec.TrustDomainBundle))
		if err != nil {
			return nil, fmt.Errorf("invalid trustDomainBundle value: %w", err)
		}
	}

	return &spireapi.FederationRelationship{
		TrustDomain:           trustDomain,
		BundleEndpointURL:     bundleEndpointURL,
		BundleEndpointProfile: bundleEndpointProfile,
		TrustDomainBundle:     trustDomainBundle,
	}, nil
}

func parseBundleEndpointURL(s string) (*url.URL, error) {
	u, err := url.Parse(s)
	switch {
	case err != nil:
		return nil, err
	case u.Scheme != "https":
		return nil, errors.New("scheme must be https")
	case u.Host == "":
		return nil, errors.New("host is not specified")
	case u.User != nil:
		return nil, errors.New("cannot contain userinfo")
	}
	return u, nil
}
