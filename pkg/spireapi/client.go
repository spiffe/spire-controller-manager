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
	"fmt"
	"io"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client interface {
	EntryClient
	TrustDomainClient
	SVIDClient
	BundleClient
	io.Closer
}

func DialSocket(path string) (Client, error) {
	var target string
	if filepath.IsAbs(path) {
		target = "unix://" + path
	} else {
		target = "unix:" + path
	}

	grpcClient, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to dial API socket: %w", err)
	}

	return struct {
		EntryClient
		TrustDomainClient
		SVIDClient
		BundleClient
		io.Closer
	}{
		EntryClient:       NewEntryClient(grpcClient),
		TrustDomainClient: NewTrustDomainClient(grpcClient),
		SVIDClient:        NewSVIDClient(grpcClient),
		BundleClient:      NewBundleClient(grpcClient),
		Closer:            grpcClient,
	}, nil
}
