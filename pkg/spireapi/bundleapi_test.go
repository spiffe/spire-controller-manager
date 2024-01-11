package spireapi

import (
	"context"
	"sync"
	"testing"

	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	bundlev1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/bundle/v1"
	apitypes "github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	bundleNotAvailable = status.Errorf(codes.Unavailable, "bundle not available")
)

func TestBundleAPIGetBundle(t *testing.T) {
	server, client := startBundleAPIServer(t)

	bundle := spiffebundle.New(domain1)

	for _, tc := range []struct {
		desc         string
		withBundle   *spiffebundle.Bundle
		expectBundle *spiffebundle.Bundle
		expectErr    error
	}{
		{
			desc:      "bundle not available",
			expectErr: bundleNotAvailable,
		},
		{
			desc:         "success",
			withBundle:   bundle,
			expectBundle: bundle,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.withBundle != nil {
				server.setBundle(t, tc.withBundle)
			}
			actualBundle, err := client.GetBundle(ctx)
			if tc.expectErr != nil {
				assertErrorIs(t, err, tc.expectErr)
				assert.Nil(t, actualBundle)
				return
			}
			assert.NoError(t, err)
			if assert.NotNil(t, actualBundle) {
				assert.Equal(t, marshalBundle(t, tc.expectBundle), marshalBundle(t, actualBundle))
			}
		})
	}
}

func startBundleAPIServer(t *testing.T) (*bundleServer, BundleClient) {
	api := &bundleServer{}
	conn := startServer(t, func(s *grpc.Server) {
		bundlev1.RegisterBundleServer(s, api)
	})
	return api, NewBundleClient(conn)
}

type bundleServer struct {
	bundlev1.UnimplementedBundleServer

	mtx    sync.RWMutex
	bundle *apitypes.Bundle
}

func (s *bundleServer) GetBundle(_ context.Context, _ *bundlev1.GetBundleRequest) (*apitypes.Bundle, error) {
	s.mtx.RLock()
	bundle := s.bundle
	s.mtx.RUnlock()

	if bundle == nil {
		return nil, bundleNotAvailable
	}
	return bundle, nil
}

func (s *bundleServer) setBundle(t *testing.T, bundle *spiffebundle.Bundle) {
	b, err := bundleToAPI(bundle)
	require.NoError(t, err)

	s.mtx.Lock()
	s.bundle = b
	s.mtx.Unlock()
}

func marshalBundle(t *testing.T, b *spiffebundle.Bundle) string {
	d, err := b.Marshal()
	require.NoError(t, err)
	return string(d)
}
