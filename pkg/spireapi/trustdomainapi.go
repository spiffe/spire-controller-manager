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

package spireapi

import (
	"context"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	trustdomainv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/trustdomain/v1"
	apitypes "github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TrustDomainClient interface {
	GetFederationRelationship(ctx context.Context, td spiffeid.TrustDomain) (FederationRelationship, error)
	ListFederationRelationships(ctx context.Context) ([]FederationRelationship, error)
	CreateFederationRelationship(ctx context.Context, federationRelationship FederationRelationship) error
	CreateFederationRelationships(ctx context.Context, federationRelationships []FederationRelationship) ([]Status, error)
	UpdateFederationRelationship(ctx context.Context, federationRelationship FederationRelationship) error
	UpdateFederationRelationships(ctx context.Context, federationRelationships []FederationRelationship) ([]Status, error)
	DeleteFederationRelationship(ctx context.Context, td spiffeid.TrustDomain) error
	DeleteFederationRelationships(ctx context.Context, tds []spiffeid.TrustDomain) ([]Status, error)
}

func NewTrustDomainClient(conn grpc.ClientConnInterface) TrustDomainClient {
	return trustDomainClient{api: trustdomainv1.NewTrustDomainClient(conn)}
}

type trustDomainClient struct {
	api trustdomainv1.TrustDomainClient
}

func (c trustDomainClient) GetFederationRelationship(ctx context.Context, td spiffeid.TrustDomain) (FederationRelationship, error) {
	federationRelationship, err := c.api.GetFederationRelationship(ctx, &trustdomainv1.GetFederationRelationshipRequest{
		TrustDomain: td.String(),
	})
	if err != nil {
		return FederationRelationship{}, err
	}
	return federationRelationshipFromAPI(federationRelationship)
}

func (c trustDomainClient) CreateFederationRelationship(ctx context.Context, federationRelationship FederationRelationship) error {
	return singleStatus(c.CreateFederationRelationships(ctx, []FederationRelationship{federationRelationship}))
}

func (c trustDomainClient) UpdateFederationRelationship(ctx context.Context, federationRelationship FederationRelationship) error {
	return singleStatus(c.UpdateFederationRelationships(ctx, []FederationRelationship{federationRelationship}))
}

func (c trustDomainClient) DeleteFederationRelationship(ctx context.Context, td spiffeid.TrustDomain) error {
	return singleStatus(c.DeleteFederationRelationships(ctx, []spiffeid.TrustDomain{td}))
}

func (c trustDomainClient) ListFederationRelationships(ctx context.Context) ([]FederationRelationship, error) {
	var federationRelationships []*apitypes.FederationRelationship
	var pageToken string
	for {
		resp, err := c.api.ListFederationRelationships(ctx, &trustdomainv1.ListFederationRelationshipsRequest{
			PageToken: pageToken,
			PageSize:  federationRelationshipListPageSize,
		})
		if err != nil {
			return nil, err
		}
		federationRelationships = append(federationRelationships, resp.FederationRelationships...)
		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return federationRelationshipsFromAPI(federationRelationships)
}

func (c trustDomainClient) CreateFederationRelationships(ctx context.Context, federationRelationships []FederationRelationship) ([]Status, error) {
	statuses := make([]Status, 0, len(federationRelationships))
	err := runBatch(len(federationRelationships), federationRelationshipCreateBatchSize, func(start, end int) error {
		toCreate, err := federationRelationshipsToAPI(federationRelationships[start:end])
		if err != nil {
			return err
		}
		resp, err := c.api.BatchCreateFederationRelationship(ctx, &trustdomainv1.BatchCreateFederationRelationshipRequest{
			FederationRelationships: toCreate,
		})
		if err == nil {
			for _, result := range resp.Results {
				statuses = append(statuses, statusFromAPI(result.Status))
			}
		}
		return err
	})
	return statuses, err
}

func (c trustDomainClient) UpdateFederationRelationships(ctx context.Context, federationRelationships []FederationRelationship) ([]Status, error) {
	statuses := make([]Status, 0, len(federationRelationships))
	err := runBatch(len(federationRelationships), federationRelationshipUpdateBatchSize, func(start, end int) error {
		toUpdate, err := federationRelationshipsToAPI(federationRelationships[start:end])
		if err != nil {
			return err
		}
		resp, err := c.api.BatchUpdateFederationRelationship(ctx, &trustdomainv1.BatchUpdateFederationRelationshipRequest{
			FederationRelationships: toUpdate,
		})
		if err == nil {
			for _, result := range resp.Results {
				statuses = append(statuses, statusFromAPI(result.Status))
			}
		}
		return err
	})
	return statuses, err
}

func (c trustDomainClient) DeleteFederationRelationships(ctx context.Context, tds []spiffeid.TrustDomain) ([]Status, error) {
	statuses := make([]Status, 0, len(tds))
	err := runBatch(len(tds), federationRelationshipDeleteBatchSize, func(start, end int) error {
		toDelete, err := trustDomainsToAPI(tds[start:end])
		if err != nil {
			return err
		}
		resp, err := c.api.BatchDeleteFederationRelationship(ctx, &trustdomainv1.BatchDeleteFederationRelationshipRequest{
			TrustDomains: toDelete,
		})
		if err == nil {
			for _, result := range resp.Results {
				statuses = append(statuses, statusFromAPI(result.Status))
			}
		}
		return err
	})
	return statuses, err
}

func singleStatus(statuses []Status, err error) error {
	switch {
	case err != nil:
		return err
	case len(statuses) > 0:
		return statuses[0].Err()
	default:
		return status.Error(codes.Internal, "status was unexpectedly not returned")
	}
}
