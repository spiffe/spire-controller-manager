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
)

type TrustDomainClient interface {
	ListFederationRelationships(ctx context.Context) ([]FederationRelationship, error)
	CreateFederationRelationships(ctx context.Context, federationRelationships []FederationRelationship) ([]Status, error)
	UpdateFederationRelationships(ctx context.Context, federationRelationships []FederationRelationship) ([]Status, error)
	DeleteFederationRelationships(ctx context.Context, tds []spiffeid.TrustDomain) ([]Status, error)
}

func NewTrustDomainClient(conn grpc.ClientConnInterface) TrustDomainClient {
	return trustDomainClient{api: trustdomainv1.NewTrustDomainClient(conn)}
}

type trustDomainClient struct {
	api trustdomainv1.TrustDomainClient
}

func (c trustDomainClient) ListFederationRelationships(ctx context.Context) ([]FederationRelationship, error) {
	var federationRelationships []*apitypes.FederationRelationship
	var pageToken string
	for {
		resp, err := c.api.ListFederationRelationships(ctx, &trustdomainv1.ListFederationRelationshipsRequest{
			PageToken: pageToken,
			PageSize:  int32(federationRelationshipListPageSize),
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
	var statuses []Status
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
	var statuses []Status
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
	var statuses []Status
	err := runBatch(len(tds), federationRelationshipDeleteBatchSize, func(start, end int) error {
		resp, err := c.api.BatchDeleteFederationRelationship(ctx, &trustdomainv1.BatchDeleteFederationRelationshipRequest{
			TrustDomains: trustDomainsToAPI(tds[start:end]),
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
