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

	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	bundlev1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/bundle/v1"
	"google.golang.org/grpc"
)

type BundleClient interface {
	// MintX509SVID mints an X509-SVID
	GetBundle(ctx context.Context) (*spiffebundle.Bundle, error)
}

func NewBundleClient(conn grpc.ClientConnInterface) BundleClient {
	return bundleClient{api: bundlev1.NewBundleClient(conn)}
}

type bundleClient struct {
	api bundlev1.BundleClient
}

func (c bundleClient) GetBundle(ctx context.Context) (*spiffebundle.Bundle, error) {
	bundle, err := c.api.GetBundle(ctx, &bundlev1.GetBundleRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get bundle: %w", err)
	}

	return bundleFromAPI(bundle)
}
