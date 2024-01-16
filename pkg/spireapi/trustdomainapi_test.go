package spireapi

import (
	"context"
	"sort"
	"sync"
	"testing"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	trustdomainv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/trustdomain/v1"
	apitypes "github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	domain1FR = FederationRelationship{
		TrustDomain:           domain1,
		BundleEndpointURL:     "https://domain1.test/bundle",
		BundleEndpointProfile: HTTPSWebProfile{},
	}

	domain2FR = FederationRelationship{
		TrustDomain:           domain2,
		BundleEndpointURL:     "https://domain2.test/bundle",
		BundleEndpointProfile: HTTPSWebProfile{},
	}

	domain3FR = FederationRelationship{
		TrustDomain:           domain3,
		BundleEndpointURL:     "https://domain3.test/bundle",
		BundleEndpointProfile: HTTPSWebProfile{},
	}
)

func init() {
	federationRelationshipCreateBatchSize = 2
	federationRelationshipUpdateBatchSize = 2
	federationRelationshipDeleteBatchSize = 2
	federationRelationshipListPageSize = 2
}

func TestTrustDomainAPIListFederationRelationships(t *testing.T) {
	server, client := startTrustDomainAPIServer(t)

	for _, tc := range []struct {
		desc      string
		expectFRs []FederationRelationship
		expectErr error
	}{
		{
			desc:      "error",
			expectErr: status.Error(codes.Internal, "oh no"),
		},
		{
			desc:      "empty",
			expectFRs: nil,
		},
		{
			desc:      "less than a page",
			expectFRs: []FederationRelationship{domain1FR},
		},
		{
			desc:      "exactly a page",
			expectFRs: []FederationRelationship{domain1FR, domain2FR},
		},
		{
			desc:      "more than a page",
			expectFRs: []FederationRelationship{domain1FR, domain2FR, domain3FR},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			server.listFederationRelationshipsErr = tc.expectErr
			server.setFederationRelationships(t, tc.expectFRs...)
			actualFRs, err := client.ListFederationRelationships(ctx)
			if tc.expectErr != nil {
				assertErrorIs(t, err, tc.expectErr)
				assert.Nil(t, actualFRs)
				return
			}
			assert.NoError(t, err)
			assert.ElementsMatch(t, tc.expectFRs, actualFRs)
		})
	}
}

func TestCreateFederationRelationships(t *testing.T) {
	server, client := startTrustDomainAPIServer(t)

	ok := Status{Code: codes.OK}

	for _, tc := range []struct {
		desc         string
		withFRs      []FederationRelationship
		createFRs    []FederationRelationship
		expectFRs    []FederationRelationship
		expectStatus []Status
		expectErr    error
	}{
		{
			desc:         "empty",
			expectFRs:    nil,
			expectStatus: nil,
		},
		{
			desc:      "RPC error",
			createFRs: []FederationRelationship{domain1FR},
			expectErr: status.Error(codes.Internal, "oh no"),
		},
		{
			desc:         "already exists",
			withFRs:      []FederationRelationship{domain1FR},
			createFRs:    []FederationRelationship{domain1FR},
			expectFRs:    []FederationRelationship{domain1FR},
			expectStatus: []Status{{Code: codes.AlreadyExists, Message: `federation relationship "domain1" already exists`}},
		},
		{
			desc:         "less than a batch",
			createFRs:    []FederationRelationship{domain1FR},
			expectFRs:    []FederationRelationship{domain1FR},
			expectStatus: []Status{ok},
		},
		{
			desc:         "exactly a batch",
			createFRs:    []FederationRelationship{domain1FR, domain2FR},
			expectFRs:    []FederationRelationship{domain1FR, domain2FR},
			expectStatus: []Status{ok, ok},
		},
		{
			desc:         "more than a batch",
			createFRs:    []FederationRelationship{domain1FR, domain2FR, domain3FR},
			expectFRs:    []FederationRelationship{domain1FR, domain2FR, domain3FR},
			expectStatus: []Status{ok, ok, ok},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			server.setFederationRelationships(t, tc.withFRs...)
			server.batchCreateFederationRelationshipsErr = tc.expectErr
			actualStatus, err := client.CreateFederationRelationships(ctx, tc.createFRs)
			if tc.expectErr != nil {
				assertErrorIs(t, err, tc.expectErr)
				assert.Nil(t, actualStatus)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectStatus, actualStatus)
			assert.ElementsMatch(t, tc.expectFRs, server.getFederationRelationships(t))
		})
	}
}

func TestUpdateFederationRelationships(t *testing.T) {
	server, client := startTrustDomainAPIServer(t)

	ok := Status{Code: codes.OK}

	dupWithURL := func(fr FederationRelationship, url string) FederationRelationship {
		fr.BundleEndpointURL = url
		return fr
	}

	domain1FROld := dupWithURL(domain1FR, "https://whatever.test/replace/me/1")
	domain2FROld := dupWithURL(domain2FR, "https://whatever.test/replace/me/2")
	domain3FROld := dupWithURL(domain3FR, "https://whatever.test/replace/me/3")

	for _, tc := range []struct {
		desc         string
		withFRs      []FederationRelationship
		updateFRs    []FederationRelationship
		expectFRs    []FederationRelationship
		expectStatus []Status
		expectErr    error
	}{
		{
			desc:         "empty",
			expectFRs:    nil,
			expectStatus: nil,
		},
		{
			desc:      "RPC error",
			updateFRs: []FederationRelationship{domain1FR},
			expectErr: status.Error(codes.Internal, "oh no"),
		},
		{
			desc:         "not found",
			updateFRs:    []FederationRelationship{domain1FR},
			expectStatus: []Status{{Code: codes.NotFound, Message: `federation relationship "domain1" not found`}},
		},
		{
			desc:         "less than a batch",
			withFRs:      []FederationRelationship{domain1FROld},
			updateFRs:    []FederationRelationship{domain1FR},
			expectFRs:    []FederationRelationship{domain1FR},
			expectStatus: []Status{ok},
		},
		{
			desc:         "exactly a batch",
			withFRs:      []FederationRelationship{domain1FROld, domain2FROld},
			updateFRs:    []FederationRelationship{domain1FR, domain2FR},
			expectFRs:    []FederationRelationship{domain1FR, domain2FR},
			expectStatus: []Status{ok, ok},
		},
		{
			desc:         "more than a batch",
			withFRs:      []FederationRelationship{domain1FROld, domain2FROld, domain3FROld},
			updateFRs:    []FederationRelationship{domain1FR, domain2FR, domain3FR},
			expectFRs:    []FederationRelationship{domain1FR, domain2FR, domain3FR},
			expectStatus: []Status{ok, ok, ok},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			server.setFederationRelationships(t, tc.withFRs...)
			server.batchUpdateFederationRelationshipsErr = tc.expectErr
			actualStatus, err := client.UpdateFederationRelationships(ctx, tc.updateFRs)
			if tc.expectErr != nil {
				assertErrorIs(t, err, tc.expectErr)
				assert.Nil(t, actualStatus)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectStatus, actualStatus)
			assert.ElementsMatch(t, tc.expectFRs, server.getFederationRelationships(t))
		})
	}
}

func TestDeleteFederationRelationships(t *testing.T) {
	server, client := startTrustDomainAPIServer(t)

	ok := Status{Code: codes.OK}

	for _, tc := range []struct {
		desc         string
		withFRs      []FederationRelationship
		deleteFRs    []spiffeid.TrustDomain
		expectFRs    []FederationRelationship
		expectStatus []Status
		expectErr    error
	}{
		{
			desc:         "empty",
			expectFRs:    nil,
			expectStatus: nil,
		},
		{
			desc:      "RPC error",
			deleteFRs: []spiffeid.TrustDomain{domain1},
			expectErr: status.Error(codes.Internal, "oh no"),
		},
		{
			desc:         "not found",
			deleteFRs:    []spiffeid.TrustDomain{domain1},
			expectStatus: []Status{{Code: codes.NotFound, Message: `federation relationship "domain1" not found`}},
		},
		{
			desc:         "less than a batch",
			withFRs:      []FederationRelationship{domain1FR},
			deleteFRs:    []spiffeid.TrustDomain{domain1},
			expectStatus: []Status{ok},
		},
		{
			desc:         "exactly a batch",
			withFRs:      []FederationRelationship{domain1FR, domain2FR},
			deleteFRs:    []spiffeid.TrustDomain{domain1, domain2},
			expectStatus: []Status{ok, ok},
		},
		{
			desc:         "more than a batch",
			withFRs:      []FederationRelationship{domain1FR, domain2FR, domain3FR},
			deleteFRs:    []spiffeid.TrustDomain{domain1, domain2, domain3},
			expectStatus: []Status{ok, ok, ok},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			server.setFederationRelationships(t, tc.withFRs...)
			server.batchDeleteFederationRelationshipsErr = tc.expectErr
			actualStatus, err := client.DeleteFederationRelationships(ctx, tc.deleteFRs)
			if tc.expectErr != nil {
				assertErrorIs(t, err, tc.expectErr)
				assert.Nil(t, actualStatus)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectStatus, actualStatus)
			assert.ElementsMatch(t, tc.expectFRs, server.getFederationRelationships(t))
		})
	}
}

func startTrustDomainAPIServer(t *testing.T) (*trustDomainServer, TrustDomainClient) {
	api := &trustDomainServer{}
	conn := startServer(t, func(s *grpc.Server) {
		trustdomainv1.RegisterTrustDomainServer(s, api)
	})
	return api, NewTrustDomainClient(conn)
}

type trustDomainServer struct {
	trustdomainv1.UnimplementedTrustDomainServer

	mtx sync.RWMutex
	frs []*apitypes.FederationRelationship

	listFederationRelationshipsErr        error
	batchCreateFederationRelationshipsErr error
	batchUpdateFederationRelationshipsErr error
	batchDeleteFederationRelationshipsErr error
}

func (s *trustDomainServer) ListFederationRelationships(_ context.Context, req *trustdomainv1.ListFederationRelationshipsRequest) (*trustdomainv1.ListFederationRelationshipsResponse, error) {
	resp := new(trustdomainv1.ListFederationRelationshipsResponse)

	s.mtx.RLock()
	defer s.mtx.RUnlock()

	start, end, more := listBounds(req.PageToken, int(req.PageSize), len(s.frs), func(i int) string { return s.frs[i].TrustDomain })
	for _, fr := range s.frs[start:end] {
		resp.FederationRelationships = append(resp.FederationRelationships, fr)
		if more {
			resp.NextPageToken = fr.TrustDomain
		}
	}

	return resp, s.listFederationRelationshipsErr
}

func (s *trustDomainServer) BatchCreateFederationRelationship(_ context.Context, req *trustdomainv1.BatchCreateFederationRelationshipRequest) (*trustdomainv1.BatchCreateFederationRelationshipResponse, error) {
	resp := new(trustdomainv1.BatchCreateFederationRelationshipResponse)

	for _, fr := range req.FederationRelationships {
		st := status.Convert(s.createFederationRelationship(fr))
		result := &trustdomainv1.BatchCreateFederationRelationshipResponse_Result{
			Status: &apitypes.Status{
				Code:    int32(st.Code()),
				Message: st.Message(),
			},
		}
		if st.Code() == codes.OK {
			result.FederationRelationship = fr
		}
		resp.Results = append(resp.Results, result)
	}

	return resp, s.batchCreateFederationRelationshipsErr
}

func (s *trustDomainServer) BatchUpdateFederationRelationship(_ context.Context, req *trustdomainv1.BatchUpdateFederationRelationshipRequest) (*trustdomainv1.BatchUpdateFederationRelationshipResponse, error) {
	resp := new(trustdomainv1.BatchUpdateFederationRelationshipResponse)

	for _, fr := range req.FederationRelationships {
		st := status.Convert(s.updateFederationRelationship(fr))
		result := &trustdomainv1.BatchUpdateFederationRelationshipResponse_Result{
			Status: &apitypes.Status{
				Code:    int32(st.Code()),
				Message: st.Message(),
			},
		}
		if st.Code() == codes.OK {
			result.FederationRelationship = fr
		}
		resp.Results = append(resp.Results, result)
	}
	return resp, s.batchUpdateFederationRelationshipsErr
}

func (s *trustDomainServer) BatchDeleteFederationRelationship(_ context.Context, req *trustdomainv1.BatchDeleteFederationRelationshipRequest) (*trustdomainv1.BatchDeleteFederationRelationshipResponse, error) {
	resp := new(trustdomainv1.BatchDeleteFederationRelationshipResponse)

	for _, td := range req.TrustDomains {
		st := status.Convert(s.deleteFederationRelationship(td))
		result := &trustdomainv1.BatchDeleteFederationRelationshipResponse_Result{
			Status: &apitypes.Status{
				Code:    int32(st.Code()),
				Message: st.Message(),
			},
			TrustDomain: td,
		}
		resp.Results = append(resp.Results, result)
	}
	return resp, s.batchDeleteFederationRelationshipsErr
}

func (s *trustDomainServer) clearFederationRelationships() {
	s.mtx.Lock()
	s.frs = nil
	s.mtx.Unlock()
}

func (s *trustDomainServer) getFederationRelationships(t *testing.T) []FederationRelationship {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	frs, err := federationRelationshipsFromAPI(s.frs)
	require.NoError(t, err)
	return frs
}

func (s *trustDomainServer) setFederationRelationships(t *testing.T, frs ...FederationRelationship) {
	s.clearFederationRelationships()
	for _, fr := range frs {
		api, err := federationRelationshipToAPI(fr)
		require.NoError(t, err)
		err = s.createFederationRelationship(api)
		require.NoError(t, err, "test setup failure creating federation relationship")
	}
}

func (s *trustDomainServer) createFederationRelationship(fr *apitypes.FederationRelationship) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	n := sort.Search(len(s.frs), func(i int) bool {
		return s.frs[i].TrustDomain >= fr.TrustDomain
	})
	if n < len(s.frs) && s.frs[n].TrustDomain == fr.TrustDomain {
		return status.Errorf(codes.AlreadyExists, "federation relationship %q already exists", fr.TrustDomain)
	}
	s.frs = append(s.frs[:n], append([]*apitypes.FederationRelationship{fr}, s.frs[n:]...)...)
	return nil
}

func (s *trustDomainServer) updateFederationRelationship(fr *apitypes.FederationRelationship) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	n := sort.Search(len(s.frs), func(i int) bool {
		return s.frs[i].TrustDomain >= fr.TrustDomain
	})
	if !(n < len(s.frs) && s.frs[n].TrustDomain == fr.TrustDomain) {
		return status.Errorf(codes.NotFound, "federation relationship %q not found", fr.TrustDomain)
	}
	s.frs[n] = fr
	return nil
}

func (s *trustDomainServer) deleteFederationRelationship(td string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	n := sort.Search(len(s.frs), func(i int) bool {
		return s.frs[i].TrustDomain >= td
	})
	if !(n < len(s.frs) && s.frs[n].TrustDomain == td) {
		return status.Errorf(codes.NotFound, "federation relationship %q not found", td)
	}
	s.frs = s.frs[:n+copy(s.frs[n:], s.frs[n+1:])]
	return nil
}
