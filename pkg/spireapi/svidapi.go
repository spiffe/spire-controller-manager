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
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	svidv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/svid/v1"
	"google.golang.org/grpc"
)

const (
	DefaultX509SVIDTTL = time.Hour
)

type X509SVID struct {
	// ID is the SPIFFE ID of the X509-SVID.
	ID spiffeid.ID

	// Key is the private key of the X509-SVID
	Key crypto.Signer

	// CertChain contains the X509-SVID and any intermediates required to
	// chain back to trusted root in the trust domain bundle. The X509-SVID
	// is the first certificate in the chain.
	CertChain []*x509.Certificate

	// ExpiresAt contains the expiration time of the X509-SVID.
	ExpiresAt time.Time
}

type X509SVIDParams struct {
	// Key is the X509-SVID private key.
	Key crypto.Signer

	// ID is the SPIFFE ID of the X509-SVID. Required.
	ID spiffeid.ID

	// DNSNames are optional DNS SANs to add to the X509-SVID. Optional.
	DNSNames []string

	// Subject is the Subject of the X509-SVID. Optional.
	Subject pkix.Name

	// TTL is the requested time-to-live. The actual TTL may be smaller than
	// requested. Optional. If unset, the TTL is at most one hour.
	TTL time.Duration
}

type SVIDClient interface {
	// MintX509SVID mints an X509-SVID
	MintX509SVID(ctx context.Context, params X509SVIDParams) (*X509SVID, error)
}

func NewSVIDClient(conn grpc.ClientConnInterface) SVIDClient {
	return svidClient{api: svidv1.NewSVIDClient(conn)}
}

type svidClient struct {
	api svidv1.SVIDClient
}

func (c svidClient) MintX509SVID(ctx context.Context, params X509SVIDParams) (*X509SVID, error) {
	switch {
	case params.Key == nil:
		return nil, errors.New("key is required")
	case params.ID.IsZero():
		return nil, errors.New("id is required")
	case params.TTL < 0:
		return nil, errors.New("negative TTL is not allowed")
	case params.TTL == 0:
		params.TTL = DefaultX509SVIDTTL
	}

	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:  params.Subject,
		DNSNames: params.DNSNames,
		URIs:     []*url.URL{params.ID.URL()},
	}, params.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create X509-SVID CSR: %w", err)
	}

	resp, err := c.api.MintX509SVID(ctx, &svidv1.MintX509SVIDRequest{
		Csr: csr,
		Ttl: int32(params.TTL.Seconds()),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to mint X509-SVID: %w", err)
	}

	if resp.Svid == nil {
		return nil, errors.New("no X509-SVID in response")
	}

	td, err := spiffeid.TrustDomainFromString(resp.Svid.Id.TrustDomain)
	if err != nil {
		return nil, fmt.Errorf("invalid trust domain in response ID: %w", err)
	}

	id, err := spiffeid.FromPath(td, resp.Svid.Id.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid SPIFFE ID in response: %w", err)
	}

	var certChain []*x509.Certificate
	for _, certDER := range resp.Svid.CertChain {
		cert, err := x509.ParseCertificate(certDER)
		if err != nil {
			return nil, fmt.Errorf("invalid certificate in response: %w", err)
		}
		certChain = append(certChain, cert)
	}
	if len(certChain) == 0 {
		return nil, errors.New("no certificates in response")
	}

	return &X509SVID{
		ID:        id,
		Key:       params.Key,
		CertChain: certChain,
		ExpiresAt: certChain[0].NotAfter,
	}, nil
}
