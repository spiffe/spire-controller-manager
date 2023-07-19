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
	"fmt"

	entryv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/entry/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	apitypes "github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

type EntryClient interface {
	ListEntries(ctx context.Context) ([]Entry, error)
	CreateEntries(ctx context.Context, entries []Entry) ([]Status, error)
	UpdateEntries(ctx context.Context, entries []Entry) ([]Status, error)
	DeleteEntries(ctx context.Context, entryIDs []string) ([]Status, error)
	GetUnsupportedFields(ctx context.Context, td string) (map[string]struct{}, error)
}

func NewEntryClient(conn grpc.ClientConnInterface) EntryClient {
	return entryClient{api: entryv1.NewEntryClient(conn)}
}

type entryClient struct {
	api entryv1.EntryClient
}

func (c entryClient) ListEntries(ctx context.Context) ([]Entry, error) {
	var entries []*apitypes.Entry
	var pageToken string
	for {
		resp, err := c.api.ListEntries(ctx, &entryv1.ListEntriesRequest{
			PageToken: pageToken,
			PageSize:  int32(entryListPageSize),
		})
		if err != nil {
			return nil, err
		}
		entries = append(entries, resp.Entries...)
		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return entriesFromAPI(entries)
}

func (c entryClient) GetUnsupportedFields(ctx context.Context, td string) (map[string]struct{}, error) {
	unsupportedFields := make(map[string]struct{})
	resp, err := c.api.BatchCreateEntry(ctx, &entryv1.BatchCreateEntryRequest{
		Entries: []*apitypes.Entry{
			{
				ParentId: &types.SPIFFEID{
					TrustDomain: td,
					Path:        "/dummyagent",
				},
				SpiffeId: &types.SPIFFEID{
					TrustDomain: td,
					Path:        "/w1",
				},
				Selectors: []*types.Selector{
					{
						Type:  "a",
						Value: "1",
					},
				},
				X509SvidTtl: 60,
				JwtSvidTtl:  60,
				Hint:        "hint",
			},
		},
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Results) != 1 {
		return nil, fmt.Errorf("only one response expected but got %v", len(resp.Results))
	}

	result := resp.Results[0]
	if result.Status.Code != int32(codes.OK) {
		return nil, fmt.Errorf("failed to create entry: %v", result.Status.Message)
	}

	deleteResponse, err := c.api.BatchDeleteEntry(ctx, &entryv1.BatchDeleteEntryRequest{
		Ids: []string{
			result.Entry.Id,
		},
	})
	if err != nil {
		return nil, err
	}

	if len(deleteResponse.Results) != 1 {
		return nil, fmt.Errorf("only one response expected when deleting entry but got %v", len(resp.Results))
	}

	deleteResult := deleteResponse.Results[0]
	if deleteResult.Status.Code != int32(codes.OK) {
		return nil, fmt.Errorf("failed to create entry: %v", deleteResult.Status.Message)
	}

	if result.Entry.JwtSvidTtl == 0 {
		unsupportedFields["jwtSVIDTTL"] = struct{}{}
	}

	if result.Entry.Hint == "" {
		unsupportedFields["hint"] = struct{}{}
	}

	return unsupportedFields, nil
}

func (c entryClient) CreateEntries(ctx context.Context, entries []Entry) ([]Status, error) {
	statuses := make([]Status, 0, len(entries))
	err := runBatch(len(entries), entryCreateBatchSize, func(start, end int) error {
		resp, err := c.api.BatchCreateEntry(ctx, &entryv1.BatchCreateEntryRequest{
			Entries: entriesToAPI(entries[start:end]),
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

func (c entryClient) UpdateEntries(ctx context.Context, entries []Entry) ([]Status, error) {
	statuses := make([]Status, 0, len(entries))
	err := runBatch(len(entries), entryUpdateBatchSize, func(start, end int) error {
		resp, err := c.api.BatchUpdateEntry(ctx, &entryv1.BatchUpdateEntryRequest{
			Entries: entriesToAPI(entries[start:end]),
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

func (c entryClient) DeleteEntries(ctx context.Context, entryIDs []string) ([]Status, error) {
	statuses := make([]Status, 0, len(entryIDs))
	err := runBatch(len(entryIDs), entryDeleteBatchSize, func(start, end int) error {
		resp, err := c.api.BatchDeleteEntry(ctx, &entryv1.BatchDeleteEntryRequest{
			Ids: entryIDs[start:end],
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
