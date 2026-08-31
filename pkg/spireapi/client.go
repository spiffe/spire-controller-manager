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
	"io"
	"path/filepath"

	"github.com/spiffe/go-spiffe/v2/spiffegrpc/grpccredentials"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
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

type GrpcConfig struct {
	// MaxCallRecvMsgSize is the maximum message size the controller manager will receive.
	MaxCallRecvMsgSize int `json:"maxCallRecvMsgSize,omitempty"`
}

func DialSocket(path string, grpcConfig *GrpcConfig) (Client, error) {
	var target string
	if filepath.IsAbs(path) {
		target = "unix://" + path
	} else {
		target = "unix:" + path
	}
	grpcOptions := append([]grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}, getCallOptions(grpcConfig)...)
	grpcOptions = append(grpcOptions, grpc.WithDefaultCallOptions(grpc.WaitForReady(true)))

	grpcClient, err := grpc.NewClient(target, grpcOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial API socket: %w", err)
	}

	return newClient(grpcClient, grpcClient), nil
}

// DialAddress dials the SPIRE Server API over a TCP address using SPIFFE
// mTLS. The controller manager obtains its own X509-SVID via the SPIFFE
// Workload API, reachable at workloadAPIAddr (e.g.
// "unix:///spiffe-workload-api/spire-agent.sock"), and uses it, along with
// the X.509 bundle for trustDomain, to authenticate the SPIRE Server and
// establish an mTLS connection to it.
//
// The SPIRE Server must be configured to grant the caller's SPIFFE ID admin
// rights, either via a registration entry with the admin flag set, or via
// the server's admin_ids configuration.
func DialAddress(ctx context.Context, addr string, trustDomain spiffeid.TrustDomain, workloadAPIAddr string, grpcConfig *GrpcConfig) (Client, error) {
	var clientOptions []workloadapi.ClientOption
	if workloadAPIAddr != "" {
		clientOptions = append(clientOptions, workloadapi.WithAddr(workloadAPIAddr))
	}

	source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(clientOptions...))
	if err != nil {
		return nil, fmt.Errorf("failed to create X509Source: %w", err)
	}

	creds := grpccredentials.MTLSClientCredentials(source, source, tlsconfig.AuthorizeMemberOf(trustDomain))

	grpcOptions := append([]grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}, getCallOptions(grpcConfig)...)
	grpcOptions = append(grpcOptions, grpc.WithDefaultCallOptions(grpc.WaitForReady(true)))

	grpcClient, err := grpc.NewClient(addr, grpcOptions...)
	if err != nil {
		_ = source.Close()
		return nil, fmt.Errorf("failed to dial SPIRE Server address: %w", err)
	}

	return newClient(grpcClient, multiCloser{grpcClient, source}), nil
}

func newClient(cc grpc.ClientConnInterface, closer io.Closer) Client {
	return struct {
		EntryClient
		TrustDomainClient
		SVIDClient
		BundleClient
		io.Closer
	}{
		EntryClient:       NewEntryClient(cc),
		TrustDomainClient: NewTrustDomainClient(cc),
		SVIDClient:        NewSVIDClient(cc),
		BundleClient:      NewBundleClient(cc),
		Closer:            closer,
	}
}

// multiCloser closes multiple io.Closers, returning the first error
// encountered (if any) while still attempting to close all of them.
type multiCloser []io.Closer

func (m multiCloser) Close() error {
	var firstErr error
	for _, closer := range m {
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func getCallOptions(grpcConfig *GrpcConfig) []grpc.DialOption {
	var grpcOptions []grpc.DialOption

	if grpcConfig != nil {
		callOptions := []grpc.CallOption{}
		if grpcConfig.MaxCallRecvMsgSize > 0 {
			callOptions = append(callOptions, grpc.MaxCallRecvMsgSize(grpcConfig.MaxCallRecvMsgSize))
		}
		if len(callOptions) > 0 {
			grpcOptions = append(grpcOptions, grpc.WithDefaultCallOptions(callOptions...))
		}
	}

	return grpcOptions
}
