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
	"context"
	"errors"
	"fmt"
	"text/template"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	dnsNameTemplateName          = "dnsNameTemplate"
	spiffeIDTemplateName         = "spiffeIDTemplate"
	workloadSelectorTemplateName = "workloadSelectorTemplate"
)

// log is for logging in this package.
var clusterspiffeidlog = logf.Log.WithName("clusterspiffeid-resource")

func (r *ClusterSPIFFEID) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&ClusterSPIFFEIDCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-spire-spiffe-io-v1alpha1-clusterspiffeid,mutating=false,failurePolicy=fail,sideEffects=None,groups=spire.spiffe.io,resources=clusterspiffeids,verbs=create;update,versions=v1alpha1,name=vclusterspiffeid.kb.io,admissionReviewVersions=v1

type ClusterSPIFFEIDCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &ClusterSPIFFEIDCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ClusterSPIFFEIDCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	o, ok := obj.(*ClusterSPIFFEID)
	if !ok {
		return nil, fmt.Errorf("expected a ClusterSPIFFEID object but got %T", obj)
	}
	clusterspiffeidlog.Info("validate create", "name", o.Name)

	return r.validate(o)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ClusterSPIFFEIDCustomValidator) ValidateUpdate(_ context.Context, _ runtime.Object, nobj runtime.Object) (admission.Warnings, error) {
	o, ok := nobj.(*ClusterSPIFFEID)
	if !ok {
		return nil, fmt.Errorf("expected a ClusterSPIFFEID object but got %T", nobj)
	}
	clusterspiffeidlog.Info("validate update", "name", o.Name)

	return r.validate(o)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ClusterSPIFFEIDCustomValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	// Deletes are not validated.
	return nil, nil
}

func (r *ClusterSPIFFEIDCustomValidator) validate(o *ClusterSPIFFEID) (admission.Warnings, error) {
	_, err := ParseClusterSPIFFEIDSpec(&o.Spec)
	return nil, err
}

// +kubebuilder:object:generate=false
// ParsedClusterSPIFFEIDSpec is a parsed and validated ClusterSPIFFEIDSpec
type ParsedClusterSPIFFEIDSpec struct {
	SPIFFEIDTemplate          *template.Template
	NamespaceSelector         labels.Selector
	PodSelector               labels.Selector
	TTL                       time.Duration
	JWTTTL                    time.Duration
	FederatesWith             []spiffeid.TrustDomain
	DNSNameTemplates          []*template.Template
	WorkloadSelectorTemplates []*template.Template
	Admin                     bool
	Downstream                bool
	AutoPopulateDNSNames      bool
	Hint                      string
}

// ParseClusterSPIFFEIDSpec parses and validates the fields in the ClusterSPIFFEIDSpec
func ParseClusterSPIFFEIDSpec(spec *ClusterSPIFFEIDSpec) (*ParsedClusterSPIFFEIDSpec, error) {
	if spec.SPIFFEIDTemplate == "" {
		return nil, errors.New("empty SPIFFEID template")
	}

	spiffeIDTemplate, err := template.New(spiffeIDTemplateName).Parse(spec.SPIFFEIDTemplate)
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
		dnsNameTemplate, err := template.New(dnsNameTemplateName).Parse(value)
		if err != nil {
			return nil, fmt.Errorf("invalid dnsNameTemplate value: %w", err)
		}
		dnsNameTemplates = append(dnsNameTemplates, dnsNameTemplate)
	}

	var workloadSelectorTemplates []*template.Template
	for _, value := range spec.WorkloadSelectorTemplates {
		workloadSelectorTemplate, err := template.New(workloadSelectorTemplateName).Parse(value)
		if err != nil {
			return nil, fmt.Errorf("invalid workloadSelectorTemplates value: %w", err)
		}
		workloadSelectorTemplates = append(workloadSelectorTemplates, workloadSelectorTemplate)
	}

	return &ParsedClusterSPIFFEIDSpec{
		SPIFFEIDTemplate:          spiffeIDTemplate,
		NamespaceSelector:         namespaceSelector,
		PodSelector:               podSelector,
		TTL:                       spec.TTL.Duration,
		JWTTTL:                    spec.JWTTTL.Duration,
		FederatesWith:             federatesWith,
		DNSNameTemplates:          dnsNameTemplates,
		WorkloadSelectorTemplates: workloadSelectorTemplates,
		Admin:                     spec.Admin,
		Downstream:                spec.Downstream,
		AutoPopulateDNSNames:      spec.AutoPopulateDNSNames,
		Hint:                      spec.Hint,
	}, nil
}
