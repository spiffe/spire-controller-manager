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

package spirefederationrelationship

import (
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	clusterFederatedTrustDomainLogKey = "clusterFederatedTrustDomain"
	bundleEndpointURLKey              = "bundleEndpointURL"
	bundleEndpointProfileKey          = "bundleEndpointProfile"
	conflictWithKey                   = "conflictWith"
	endpointSPIFFEIDKey               = "endpointSPIFFEID"
	trustDomainKey                    = "trustDomainKey"
)

func objectName(o metav1.Object) string {
	return (types.NamespacedName{
		Namespace: o.GetNamespace(),
		Name:      o.GetName(),
	}).String()
}

func federationRelationshipFields(fr spireapi.FederationRelationship) []interface{} {
	fields := []interface{}{
		trustDomainKey, fr.TrustDomain.String(),
		bundleEndpointURLKey, fr.BundleEndpointURL,
		bundleEndpointProfileKey, safeBundleEndpointProfileName(fr.BundleEndpointProfile),
	}
	switch profile := fr.BundleEndpointProfile.(type) {
	case spireapi.HTTPSSPIFFEProfile:
		fields = append(fields, endpointSPIFFEIDKey, profile.EndpointSPIFFEID.String())
	case spireapi.HTTPSWebProfile:
	}
	return fields
}

func safeBundleEndpointProfileName(p spireapi.BundleEndpointProfile) string {
	if p != nil {
		return p.Name()
	}
	return ""
}
