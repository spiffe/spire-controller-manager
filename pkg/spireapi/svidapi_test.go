package spireapi

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	svidv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/svid/v1"
	apitypes "github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestSVIDAPIMintX509SVID(t *testing.T) {
	server, client := startSVIDAPIServer(t)

	id := spiffeid.RequireFromString("spiffe://domain.test/workload")

	for _, tc := range []struct {
		desc           string
		params         X509SVIDParams
		mutateResponse func(*svidv1.MintX509SVIDResponse) error
		expectErr      string
	}{
		{
			desc:      "missing key",
			params:    X509SVIDParams{ID: id},
			expectErr: "key is required",
		},
		{
			desc:      "missing id",
			params:    X509SVIDParams{Key: key},
			expectErr: "id is required",
		},
		{
			desc:      "negative TTL",
			params:    X509SVIDParams{ID: id, Key: key, TTL: -time.Minute},
			expectErr: "negative TTL is not allowed",
		},
		{
			desc:      "bad DNS name",
			params:    X509SVIDParams{ID: id, Key: key, DNSNames: []string{"ハロー"}},
			expectErr: `failed to create X509-SVID CSR: x509: "ハロー" cannot be encoded as an IA5String`,
		},
		{
			desc:   "mint failure",
			params: X509SVIDParams{ID: id, Key: key},
			mutateResponse: func(*svidv1.MintX509SVIDResponse) error {
				return errors.New("oh no")
			},
			expectErr: `failed to mint X509-SVID: rpc error: code = Unknown desc = oh no`,
		},
		{
			desc:   "no X509-SVID in response",
			params: X509SVIDParams{ID: id, Key: key},
			mutateResponse: func(resp *svidv1.MintX509SVIDResponse) error {
				resp.Svid = nil
				return nil
			},
			expectErr: `no X509-SVID in response`,
		},
		{
			desc:   "invalid ID trust domain",
			params: X509SVIDParams{ID: id, Key: key},
			mutateResponse: func(resp *svidv1.MintX509SVIDResponse) error {
				resp.Svid.Id.TrustDomain = ""
				return nil
			},
			expectErr: `invalid trust domain in response ID: trust domain is missing`,
		},
		{
			desc:   "invalid ID path",
			params: X509SVIDParams{ID: id, Key: key},
			mutateResponse: func(resp *svidv1.MintX509SVIDResponse) error {
				resp.Svid.Id.Path = "no-leading-slash"
				return nil
			},
			expectErr: `invalid SPIFFE ID in response: path must have a leading slash`,
		},
		{
			desc:   "empty certificate chain",
			params: X509SVIDParams{ID: id, Key: key},
			mutateResponse: func(resp *svidv1.MintX509SVIDResponse) error {
				resp.Svid.CertChain = nil
				return nil
			},
			expectErr: `no certificates in response`,
		},
		{
			desc:   "invalid certificate chain",
			params: X509SVIDParams{ID: id, Key: key},
			mutateResponse: func(resp *svidv1.MintX509SVIDResponse) error {
				resp.Svid.CertChain[0] = nil
				return nil
			},
			expectErr: `invalid certificate in response: x509: malformed certificate`,
		},
		{
			desc:   "success with default TTL",
			params: X509SVIDParams{ID: id, Key: key},
		},
		{
			desc:   "success with explicit TTL",
			params: X509SVIDParams{ID: id, Key: key, TTL: time.Hour},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			server.mutateResponse = tc.mutateResponse
			svid, err := client.MintX509SVID(ctx, tc.params)
			if tc.expectErr != "" {
				assert.EqualError(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.params.ID, svid.ID)

			expectExpiresAt := now.Add(tc.params.TTL)
			if tc.params.TTL == 0 {
				expectExpiresAt = now.Add(DefaultX509SVIDTTL)
			}
			assert.Equal(t, expectExpiresAt, svid.ExpiresAt)
		})
	}
}

func startSVIDAPIServer(t *testing.T) (*svidServer, SVIDClient) {
	api := &svidServer{}
	conn := startServer(t, func(s *grpc.Server) {
		svidv1.RegisterSVIDServer(s, api)
	})
	return api, NewSVIDClient(conn)
}

type svidServer struct {
	svidv1.UnimplementedSVIDServer
	mutateResponse func(*svidv1.MintX509SVIDResponse) error
}

func (s *svidServer) MintX509SVID(ctx context.Context, req *svidv1.MintX509SVIDRequest) (*svidv1.MintX509SVIDResponse, error) {
	csr, err := x509.ParseCertificateRequest(req.Csr)
	if err != nil {
		return nil, err
	}
	if len(csr.URIs) != 1 {
		return nil, fmt.Errorf("expecting one URI SAN; got %d", len(csr.URIs))
	}
	id, err := spiffeid.FromURI(csr.URIs[0])
	if err != nil {
		return nil, err
	}
	notAfter := now.Add(time.Second * time.Duration(req.Ttl))
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		URIs:         csr.URIs,
		NotAfter:     notAfter,
	}
	// Self-sign because it is easier and the client code shouldn't care.
	cert, err := createCertificate(tmpl, tmpl, csr.PublicKey, key)
	if err != nil {
		return nil, err
	}
	resp := &svidv1.MintX509SVIDResponse{
		Svid: &apitypes.X509SVID{
			CertChain: [][]byte{cert.Raw},
			Id: &apitypes.SPIFFEID{
				TrustDomain: id.TrustDomain().Name(),
				Path:        id.Path(),
			},
			ExpiresAt: notAfter.Unix(),
		},
	}
	if s.mutateResponse != nil {
		err = s.mutateResponse(resp)
	}
	return resp, err
}
