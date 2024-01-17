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

package spireentry

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var defaultParentIDTemplate *template.Template = template.Must(template.New("defaultParentIDTemplate").Parse("spiffe://{{ .TrustDomain }}/spire/agent/k8s_psat/{{ .ClusterName }}/{{ .NodeMeta.UID }}"))

func renderStaticEntry(spec *spirev1alpha1.ClusterStaticEntrySpec) (*spireapi.Entry, error) {
	spiffeID, err := spiffeid.FromString(spec.SPIFFEID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SPIFFEID: %w", err)
	}
	parentID, err := spiffeid.FromString(spec.ParentID)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ParentID: %w", err)
	}
	selectors, err := parseSelectors(spec.Selectors)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Selectors: %w", err)
	}
	federatesWith := make([]spiffeid.TrustDomain, 0, len(spec.FederatesWith))
	for _, value := range spec.FederatesWith {
		td, err := spiffeid.TrustDomainFromString(value)
		if err != nil {
			return nil, fmt.Errorf("invalid federatesWith value: %w", err)
		}
		federatesWith = append(federatesWith, td)
	}
	return &spireapi.Entry{
		SPIFFEID:      spiffeID,
		ParentID:      parentID,
		Selectors:     selectors,
		X509SVIDTTL:   spec.X509SVIDTTL.Duration,
		JWTSVIDTTL:    spec.JWTSVIDTTL.Duration,
		FederatesWith: federatesWith,
		DNSNames:      spec.DNSNames,
		Admin:         spec.Admin,
		Downstream:    spec.Downstream,
		Hint:          spec.Hint,
	}, nil
}

func renderPodEntry(spec *spirev1alpha1.ParsedClusterSPIFFEIDSpec, node *corev1.Node, pod *corev1.Pod, endpointsList *corev1.EndpointsList, trustDomain spiffeid.TrustDomain, clusterName, clusterDomain string, parentIDTemplate *template.Template) (*spireapi.Entry, error) {
	// We uniquely target the Pod running on the Node. The former is done
	// via the k8s:pod-uid selector, the latter via the parent ID.
	selectors := []spireapi.Selector{
		{Type: "k8s", Value: fmt.Sprintf("pod-uid:%s", pod.UID)},
	}

	data := &templateData{
		TrustDomain:   trustDomain.Name(),
		ClusterName:   clusterName,
		ClusterDomain: clusterDomain,
		NodeMeta:      &node.ObjectMeta,
		NodeSpec:      &node.Spec,
	}

	if parentIDTemplate == nil {
		parentIDTemplate = defaultParentIDTemplate
	}

	parentID, err := renderSPIFFEID(parentIDTemplate, data, trustDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to render parent ID: %w", err)
	}

	data.PodMeta = &pod.ObjectMeta
	data.PodSpec = &pod.Spec

	spiffeID, err := renderSPIFFEID(spec.SPIFFEIDTemplate, data, trustDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to render SPIFFE ID: %w", err)
	}

	dnsNamesSet := make(map[string]struct{})
	dnsNames, err := renderDNSNames(dnsNamesSet, spec.DNSNameTemplates, data)
	if err != nil {
		return nil, err
	}
	dnsNames = appendIfNotExists(dnsNames, dnsNamesSet, dnsNamesFromEndpoints(endpointsList, clusterDomain)...)

	for _, workloadSelectorTemplate := range spec.WorkloadSelectorTemplates {
		selector, err := renderSelector(workloadSelectorTemplate, data)
		if err != nil {
			return nil, fmt.Errorf("failed to render workload selector: %w", err)
		}
		selectors = append(selectors, selector)
	}

	return &spireapi.Entry{
		SPIFFEID:      spiffeID,
		ParentID:      parentID,
		Selectors:     selectors,
		X509SVIDTTL:   spec.TTL,
		JWTSVIDTTL:    spec.JWTTTL,
		FederatesWith: spec.FederatesWith,
		DNSNames:      dnsNames,
		Admin:         spec.Admin,
		Downstream:    spec.Downstream,
	}, nil
}

type templateData struct {
	TrustDomain   string
	ClusterName   string
	ClusterDomain string
	PodMeta       *metav1.ObjectMeta
	PodSpec       *corev1.PodSpec
	NodeMeta      *metav1.ObjectMeta
	NodeSpec      *corev1.NodeSpec
}

func renderSPIFFEID(tmpl *template.Template, data *templateData, expectTD spiffeid.TrustDomain) (spiffeid.ID, error) {
	rendered, err := renderTemplate(tmpl, data)
	if err != nil {
		return spiffeid.ID{}, err
	}
	id, err := spiffeid.FromString(rendered)
	if err != nil {
		return spiffeid.ID{}, fmt.Errorf("invalid SPIFFE ID: %w", err)
	}
	if id.TrustDomain() != expectTD {
		return spiffeid.ID{}, fmt.Errorf("invalid SPIFFE ID: expected trust domain %q but got %q", expectTD, id.TrustDomain())
	}
	return id, nil
}

func renderDNSNames(dnsNamesSet map[string]struct{}, dnsNameTemplates []*template.Template, data *templateData) (dnsNames []string, err error) {
	for _, dnsNameTemplate := range dnsNameTemplates {
		dnsName, err := renderDNSName(dnsNameTemplate, data)
		if err != nil {
			return nil, fmt.Errorf("failed to render DNS name: %w", err)
		}

		dnsNames = appendIfNotExists(dnsNames, dnsNamesSet, dnsName)
	}

	return dnsNames, nil
}

func renderDNSName(tmpl *template.Template, data *templateData) (string, error) {
	rendered, err := renderTemplate(tmpl, data)
	if err != nil {
		return "", err
	}
	return rendered, nil
}

func dnsNamesFromEndpoints(endpointsList *corev1.EndpointsList, clusterDomain string) []string {
	var dnsNames []string
	for _, endpoint := range endpointsList.Items {
		dnsNames = append(dnsNames,
			endpoint.Name,
			endpoint.Name+"."+endpoint.Namespace,
			endpoint.Name+"."+endpoint.Namespace+".svc",
		)
		if clusterDomain != "" {
			dnsNames = append(dnsNames, endpoint.Name+"."+endpoint.Namespace+".svc."+clusterDomain)
		}
	}

	// Sort the list to provide consistent results
	sort.Strings(dnsNames)

	return dnsNames
}

func renderSelector(tmpl *template.Template, data *templateData) (spireapi.Selector, error) {
	rendered, err := renderTemplate(tmpl, data)
	if err != nil {
		return spireapi.Selector{}, err
	}
	selector, err := parseSelector(rendered)
	if err != nil {
		return spireapi.Selector{}, fmt.Errorf("invalid workload selector %q: %w", rendered, err)
	}
	return selector, nil
}

func renderTemplate(tmpl *template.Template, data *templateData) (string, error) {
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.String(), nil
}

func parseSelectors(selectors []string) ([]spireapi.Selector, error) {
	ss := make([]spireapi.Selector, 0, len(selectors))
	for _, selector := range selectors {
		s, err := parseSelector(selector)
		if err != nil {
			return nil, err
		}
		ss = append(ss, s)
	}
	return ss, nil
}

func parseSelector(selector string) (spireapi.Selector, error) {
	parts := strings.SplitN(selector, ":", 2)
	switch {
	case len(parts) == 1:
		return spireapi.Selector{}, errors.New("expected at least one colon separate the type from the value")
	case len(parts[0]) == 0:
		return spireapi.Selector{}, errors.New("type cannot be empty")
	case len(parts[1]) == 0:
		return spireapi.Selector{}, errors.New("value cannot be empty")
	}
	return spireapi.Selector{
		Type:  parts[0],
		Value: parts[1],
	}, nil
}

func appendIfNotExists(slice []string, sliceSet map[string]struct{}, items ...string) []string {
	for _, item := range items {
		if _, exists := sliceSet[item]; !exists {
			sliceSet[item] = struct{}{}
			slice = append(slice, item)
		}
	}

	return slice
}
