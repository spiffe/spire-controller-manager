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
	"fmt"
	"strings"

	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var clusterfederatedtrustdomainlog = logf.Log.WithName("clusterfederatedtrustdomain-resource")

func (r *ClusterFederatedTrustDomain) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&ClusterFederatedTrustDomainCustomValidator{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-spire-spiffe-io-v1alpha1-clusterfederatedtrustdomain,mutating=false,failurePolicy=fail,sideEffects=None,groups=spire.spiffe.io,resources=clusterfederatedtrustdomains,verbs=create;update,versions=v1alpha1,name=vclusterfederatedtrustdomain.kb.io,admissionReviewVersions=v1

type ClusterFederatedTrustDomainCustomValidator struct {
	// TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &ClusterFederatedTrustDomainCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ClusterFederatedTrustDomainCustomValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	o, ok := obj.(*ClusterFederatedTrustDomain)
	if !ok {
		return nil, fmt.Errorf("expected a ClusterFederatedTrustDomain object but got %T", obj)
	}
	clusterfederatedtrustdomainlog.Info("validate create", "name", o.Name)
	return r.validate(o)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ClusterFederatedTrustDomainCustomValidator) ValidateUpdate(_ context.Context, _ runtime.Object, nobj runtime.Object) (admission.Warnings, error) {
	o, ok := nobj.(*ClusterFederatedTrustDomain)
	if !ok {
		return nil, fmt.Errorf("expected a ClusterFederatedTrustDomain object but got %T", nobj)
	}
	clusterfederatedtrustdomainlog.Info("validate update", "name", o.Name)
	return r.validate(o)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *ClusterFederatedTrustDomainCustomValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	// Deletes are not validated.
	return nil, nil
}

func (r *ClusterFederatedTrustDomainCustomValidator) validate(o *ClusterFederatedTrustDomain) (admission.Warnings, error) {
	_, err := ParseClusterFederatedTrustDomainSpec(&o.Spec)
	return nil, err
}

func ParseClusterFederatedTrustDomainSpec(spec *ClusterFederatedTrustDomainSpec) (*spireapi.FederationRelationship, error) {
	trustDomain, err := spiffeid.TrustDomainFromString(spec.TrustDomain)
	if err != nil {
		return nil, fmt.Errorf("invalid trustDomain value: %w", err)
	}

	if err := spireapi.ValidateBundleEndpointURL(spec.BundleEndpointURL); err != nil {
		return nil, fmt.Errorf("invalid bundleEndpointURL value: %w", err)
	}

	var bundleEndpointProfile spireapi.BundleEndpointProfile
	switch spec.BundleEndpointProfile.Type {
	case HTTPSWebProfileType:
		if spec.BundleEndpointProfile.EndpointSPIFFEID != "" {
			return nil, fmt.Errorf("invalid bundle endpoint profile endpointSPIFFEID value: not applicable to the %q profile", HTTPSWebProfileType)
		}
		bundleEndpointProfile = spireapi.HTTPSWebProfile{}
	case HTTPSSPIFFEProfileType:
		endpointSPIFFEID, err := spiffeid.FromString(spec.BundleEndpointProfile.EndpointSPIFFEID)
		if err != nil {
			return nil, fmt.Errorf("invalid bundle endpoint profile endpointSPIFFEID value: %w", err)
		}
		bundleEndpointProfile = spireapi.HTTPSSPIFFEProfile{
			EndpointSPIFFEID: endpointSPIFFEID,
		}
	default:
		return nil, fmt.Errorf("invalid bundle endpoint profile type value %q", spec.BundleEndpointProfile.Type)
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
		BundleEndpointURL:     spec.BundleEndpointURL,
		BundleEndpointProfile: bundleEndpointProfile,
		TrustDomainBundle:     trustDomainBundle,
	}, nil
}
