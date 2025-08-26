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
	grpcOptions := append(getGrpcConfig(grpcConfig), grpc.WithDefaultCallOptions(grpc.WaitForReady(true)))

	grpcClient, err := grpc.NewClient(target, grpcOptions...)
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

func getGrpcConfig(grpcConfig *GrpcConfig) []grpc.DialOption {
	grpcOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

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
