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

package spirefederationrelationship_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	logrtesting "github.com/go-logr/logr/testing"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	"github.com/spiffe/spire-controller-manager/pkg/spirefederationrelationship"
	"github.com/spiffe/spire-controller-manager/pkg/test/k8stest"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	td = spiffeid.RequireTrustDomainFromString("td")
)

func TestReconcile(t *testing.T) {
	now := time.Now()

	fr1 := spireapi.FederationRelationship{
		TrustDomain:           td,
		BundleEndpointURL:     "https://td.test/bundle",
		BundleEndpointProfile: spireapi.HTTPSWebProfile{},
	}
	fr2 := spireapi.FederationRelationship{
		TrustDomain:       td,
		BundleEndpointURL: "https://td.test/bundle",
		BundleEndpointProfile: spireapi.HTTPSSPIFFEProfile{
			EndpointSPIFFEID: spiffeid.RequireFromString("spiffe://td/bundle-endpoint"),
		},
	}
	cftd1 := &spirev1alpha1.ClusterFederatedTrustDomain{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "td",
			CreationTimestamp: metav1.Time{Time: now},
		},
		Spec: spirev1alpha1.ClusterFederatedTrustDomainSpec{
			TrustDomain:           "td",
			BundleEndpointURL:     "https://td.test/bundle",
			BundleEndpointProfile: spirev1alpha1.BundleEndpointProfile{Type: "https_web"},
		},
	}
	cftd2 := &spirev1alpha1.ClusterFederatedTrustDomain{
		ObjectMeta: metav1.ObjectMeta{
			Name: "td",
		},
		Spec: spirev1alpha1.ClusterFederatedTrustDomainSpec{
			TrustDomain:           "td",
			BundleEndpointURL:     "https://td.test/bundle",
			BundleEndpointProfile: spirev1alpha1.BundleEndpointProfile{Type: "https_spiffe", EndpointSPIFFEID: "spiffe://td/bundle-endpoint"},
		},
	}
	cftd3 := &spirev1alpha1.ClusterFederatedTrustDomain{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "conflicting",
			CreationTimestamp: metav1.Time{Time: now.Add(time.Second)},
		},
		Spec: spirev1alpha1.ClusterFederatedTrustDomainSpec{
			TrustDomain:           "td",
			BundleEndpointURL:     "https://td.test/bundle",
			BundleEndpointProfile: spirev1alpha1.BundleEndpointProfile{Type: "https_spiffe", EndpointSPIFFEID: "spiffe://td/bundle-endpoint"},
		},
	}

	for _, tt := range []struct {
		desc              string
		withObjects       []runtime.Object
		withFRs           []spireapi.FederationRelationship
		expectFRs         []spireapi.FederationRelationship
		configureTDClient func(tdc *trustDomainClient)
	}{
		{
			desc: "nothing to do",
		},
		{
			desc:        "handles list RPC failure",
			withObjects: []runtime.Object{cftd1},
			configureTDClient: func(tdc *trustDomainClient) {
				tdc.listError = errors.New("oh no")
			},
		},
		{
			desc:        "ignores invalid ClusterFederatedTrustDomain",
			withObjects: []runtime.Object{&spirev1alpha1.ClusterFederatedTrustDomain{}},
		},

		{
			desc:        "creates new federation relationship",
			withObjects: []runtime.Object{cftd1},
			expectFRs:   []spireapi.FederationRelationship{fr1},
		},
		{
			desc:        "handles create RPC failure",
			withObjects: []runtime.Object{cftd1},
			configureTDClient: func(tdc *trustDomainClient) {
				tdc.createError = errors.New("oh no")
			},
		},
		{
			desc:        "handles non-zero create status",
			withObjects: []runtime.Object{cftd1},
			configureTDClient: func(tdc *trustDomainClient) {
				tdc.createStatus[td] = spireapi.Status{Code: codes.Internal}
			},
		},
		{
			desc:        "updates existing federation relationship",
			withObjects: []runtime.Object{cftd2},
			withFRs:     []spireapi.FederationRelationship{fr1},
			expectFRs:   []spireapi.FederationRelationship{fr2},
		},
		{
			desc:        "handles update RPC failure",
			withObjects: []runtime.Object{cftd2},
			withFRs:     []spireapi.FederationRelationship{fr1},
			expectFRs:   []spireapi.FederationRelationship{fr1},
			configureTDClient: func(tdc *trustDomainClient) {
				tdc.updateError = errors.New("oh no")
			},
		},
		{
			desc:        "handles update RPC failure",
			withObjects: []runtime.Object{cftd2},
			withFRs:     []spireapi.FederationRelationship{fr1},
			expectFRs:   []spireapi.FederationRelationship{fr1},
			configureTDClient: func(tdc *trustDomainClient) {
				tdc.updateStatus[td] = spireapi.Status{Code: codes.Internal}
			},
		},
		{
			desc:    "deletes existing federation relationship",
			withFRs: []spireapi.FederationRelationship{fr1},
		},
		{
			desc:    "handles delete RPC failure",
			withFRs: []spireapi.FederationRelationship{fr1},
			configureTDClient: func(tdc *trustDomainClient) {
				tdc.deleteError = errors.New("oh no")
			},
			expectFRs: []spireapi.FederationRelationship{fr1},
		},
		{
			desc:    "handles non-zero delete status",
			withFRs: []spireapi.FederationRelationship{fr1},
			configureTDClient: func(tdc *trustDomainClient) {
				tdc.deleteStatus[td] = spireapi.Status{Code: codes.Internal}
			},
			expectFRs: []spireapi.FederationRelationship{fr1},
		},
		{
			desc:        "ignores conflicting resources",
			withObjects: []runtime.Object{cftd1, cftd3},
			expectFRs:   []spireapi.FederationRelationship{fr1},
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			tdc := newTrustDomainClient()
			for _, fr := range tt.withFRs {
				tdc.frs[fr.TrustDomain] = fr
			}
			if tt.configureTDClient != nil {
				tt.configureTDClient(tdc)
			}

			ctx := log.IntoContext(context.Background(), logrtesting.TestLogger{T: t})

			k8sClient := k8stest.NewClientBuilder().WithRuntimeObjects(tt.withObjects...).Build()
			spirefederationrelationship.Reconcile(ctx, tdc, k8sClient)
			assert.Equal(t, tt.expectFRs, tdc.getFederationRelationships())
		})
	}
}

type trustDomainClient struct {
	frs          map[spiffeid.TrustDomain]spireapi.FederationRelationship
	listError    error
	createStatus map[spiffeid.TrustDomain]spireapi.Status
	createError  error
	updateStatus map[spiffeid.TrustDomain]spireapi.Status
	updateError  error
	deleteStatus map[spiffeid.TrustDomain]spireapi.Status
	deleteError  error
}

func newTrustDomainClient() *trustDomainClient {
	return &trustDomainClient{
		frs:          make(map[spiffeid.TrustDomain]spireapi.FederationRelationship),
		createStatus: make(map[spiffeid.TrustDomain]spireapi.Status),
		updateStatus: make(map[spiffeid.TrustDomain]spireapi.Status),
		deleteStatus: make(map[spiffeid.TrustDomain]spireapi.Status),
	}
}

func (t *trustDomainClient) ListFederationRelationships(ctx context.Context) ([]spireapi.FederationRelationship, error) {
	if t.listError != nil {
		return nil, t.listError
	}
	return t.getFederationRelationships(), nil
}

func (t *trustDomainClient) CreateFederationRelationships(ctx context.Context, federationRelationships []spireapi.FederationRelationship) ([]spireapi.Status, error) {
	if t.createError != nil {
		return nil, t.createError
	}
	out := make([]spireapi.Status, 0, len(federationRelationships))
	for _, fr := range federationRelationships {
		var st spireapi.Status
		if _, exists := t.frs[fr.TrustDomain]; exists {
			st.Code = codes.AlreadyExists
		} else {
			st = t.createStatus[fr.TrustDomain]
		}
		if st.Code == codes.OK {
			t.frs[fr.TrustDomain] = fr
		}
		out = append(out, st)
	}
	return out, nil
}

func (t *trustDomainClient) UpdateFederationRelationships(ctx context.Context, federationRelationships []spireapi.FederationRelationship) ([]spireapi.Status, error) {
	if t.updateError != nil {
		return nil, t.updateError
	}
	out := make([]spireapi.Status, 0, len(federationRelationships))
	for _, fr := range federationRelationships {
		var st spireapi.Status
		if _, exists := t.frs[fr.TrustDomain]; !exists {
			st.Code = codes.NotFound
		} else {
			st = t.updateStatus[fr.TrustDomain]
		}
		if st.Code == codes.OK {
			t.frs[fr.TrustDomain] = fr
		}
		out = append(out, st)
	}
	return out, nil
}

func (t *trustDomainClient) DeleteFederationRelationships(ctx context.Context, tds []spiffeid.TrustDomain) ([]spireapi.Status, error) {
	if t.deleteError != nil {
		return nil, t.deleteError
	}
	out := make([]spireapi.Status, 0, len(tds))
	for _, td := range tds {
		var st spireapi.Status
		if _, exists := t.frs[td]; !exists {
			st.Code = codes.NotFound
		} else {
			st = t.deleteStatus[td]
		}
		if st.Code == codes.OK {
			delete(t.frs, td)
		}
		out = append(out, st)
	}
	return out, nil
}

func (t *trustDomainClient) getFederationRelationships() []spireapi.FederationRelationship {
	var out []spireapi.FederationRelationship
	for _, fr := range t.frs {
		out = append(out, fr)
	}
	sort.Slice(out, func(a, b int) bool {
		return out[a].TrustDomain.Compare(out[b].TrustDomain) < 0
	})
	return out
}
