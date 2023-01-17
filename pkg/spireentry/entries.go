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
	"strings"
	"text/template"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func renderPodEntry(spec *spirev1alpha1.ParsedClusterSPIFFEIDSpec, node *corev1.Node, pod *corev1.Pod, trustDomain spiffeid.TrustDomain, clusterName string) (*spireapi.Entry, error) {
	// We uniquely target the Pod running on the Node. The former is done
	// via the k8s:pod-uid selector, the latter via the parent ID.
	selectors := []spireapi.Selector{
		{Type: "k8s", Value: fmt.Sprintf("pod-uid:%s", pod.UID)},
	}
	parentID, err := spiffeid.FromPathf(trustDomain, "/spire/agent/k8s_psat/%s/%s", clusterName, node.UID)
	if err != nil {
		return nil, fmt.Errorf("failed to render parent ID: %w", err)
	}

	data := &templateData{
		TrustDomain: trustDomain.String(),
		ClusterName: clusterName,
		PodMeta:     &pod.ObjectMeta,
		PodSpec:     &pod.Spec,
		NodeMeta:    &node.ObjectMeta,
		NodeSpec:    &node.Spec,
	}

	spiffeID, err := renderSPIFFEID(spec.SPIFFEIDTemplate, data, trustDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to render SPIFFE ID: %w", err)
	}

	var dnsNames []string
	for _, dnsNameTemplate := range spec.DNSNameTemplates {
		dnsName, err := renderDNSName(dnsNameTemplate, data)
		if err != nil {
			return nil, fmt.Errorf("failed to render DNS name: %w", err)
		}
		dnsNames = append(dnsNames, dnsName)
	}

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
		FederatesWith: spec.FederatesWith,
		DnsNames:      dnsNames,
		Admin:         spec.Admin,
		Downstream:    spec.Downstream,
	}, nil
}

type templateData struct {
	TrustDomain string
	ClusterName string
	PodMeta     *metav1.ObjectMeta
	PodSpec     *corev1.PodSpec
	NodeMeta    *metav1.ObjectMeta
	NodeSpec    *corev1.NodeSpec
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

func renderDNSName(tmpl *template.Template, data *templateData) (string, error) {
	rendered, err := renderTemplate(tmpl, data)
	if err != nil {
		return "", err
	}
	if err := validateDNSName(rendered); err != nil {
		return "", fmt.Errorf("invalid DNS name %q: %w", rendered, err)
	}
	return rendered, nil
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

func validateDNSName(dnsName string) error {
	// TODO:
	return nil
}

func parseSelector(selector string) (spireapi.Selector, error) {
	parts := strings.SplitN(selector, ":", 2)
	switch {
	case len(parts) == 1:
		return spireapi.Selector{}, errors.New("expected at least one colon seperate the type from the value")
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
