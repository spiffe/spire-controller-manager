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
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffegrpc/grpccredentials"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const workloadAPISourceInitializationTimeout = 30 * time.Second

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

	source, cancelSource, err := newX509Source(ctx, workloadapi.WithClientOptions(clientOptions...))
	if err != nil {
		return nil, err
	}

	creds := grpccredentials.MTLSClientCredentials(source, source, tlsconfig.AuthorizeID(spireServerID(trustDomain)))

	grpcOptions := append([]grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}, getCallOptions(grpcConfig)...)
	grpcOptions = append(grpcOptions, grpc.WithDefaultCallOptions(grpc.WaitForReady(true)))

	grpcClient, err := grpc.NewClient(addr, grpcOptions...)
	if err != nil {
		_ = source.Close()
		cancelSource()
		return nil, fmt.Errorf("failed to dial SPIRE Server address: %w", err)
	}

	return newClient(grpcClient, closerFunc(func() error {
		err := errors.Join(grpcClient.Close(), source.Close())
		cancelSource()
		return err
	})), nil
}

func newX509Source(ctx context.Context, options ...workloadapi.X509SourceOption) (*workloadapi.X509Source, context.CancelFunc, error) {
	sourceCtx, cancelSource := context.WithCancel(ctx)
	timedOut := make(chan struct{})
	timer := time.AfterFunc(workloadAPISourceInitializationTimeout, func() {
		close(timedOut)
		cancelSource()
	})
	source, err := workloadapi.NewX509Source(sourceCtx, options...)
	if !timer.Stop() {
		<-timedOut
		if source != nil {
			_ = source.Close()
		}
		return nil, nil, fmt.Errorf("timed out waiting for initial X509-SVID from the Workload API after %s", workloadAPISourceInitializationTimeout)
	}
	if err != nil {
		cancelSource()
		return nil, nil, fmt.Errorf("failed to create X509Source: %w", err)
	}
	return source, cancelSource, nil
}

func spireServerID(trustDomain spiffeid.TrustDomain) spiffeid.ID {
	return spiffeid.RequireFromSegments(trustDomain, "spire", "server")
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

type closerFunc func() error

func (f closerFunc) Close() error {
	return f()
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
