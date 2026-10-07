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
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/url"
	"testing"
	"time"

	workload "github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	entryv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/entry/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func TestSPIREServerID(t *testing.T) {
	trustDomain := spiffeid.RequireTrustDomainFromString("example.org")

	assert.Equal(t, "spiffe://example.org/spire/server", spireServerID(trustDomain).String())
}

func TestDialAddressFailsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := DialAddress(ctx, "127.0.0.1:8081", spiffeid.RequireTrustDomainFromString("example.org"), "", nil)

	require.Nil(t, client)
	require.ErrorContains(t, err, "failed to create X509Source")
}

func TestDialAddress(t *testing.T) {
	trustDomain := spiffeid.RequireTrustDomainFromString("example.org")
	caCert, caKey := newTestCA(t)
	clientID := spiffeid.RequireFromPath(trustDomain, "/controller-manager")
	clientCert, clientKey := newTestSVID(t, caCert, caKey, clientID)
	serverCert, serverKey := newTestSVID(t, caCert, caKey, spireServerID(trustDomain))

	workloadAPIAddr := startWorkloadAPIServer(t, caCert, clientCert, clientKey, clientID)
	serverAddr := startSPIREAPIServer(t, caCert, serverCert, serverKey)

	client, err := DialAddress(context.Background(), serverAddr, trustDomain, workloadAPIAddr, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	entries, err := client.ListEntries(context.Background())
	require.NoError(t, err)
	require.Empty(t, entries)
}

type testWorkloadAPIServer struct {
	workload.UnimplementedSpiffeWorkloadAPIServer
	response *workload.X509SVIDResponse
}

func (s testWorkloadAPIServer) FetchX509SVID(_ *workload.X509SVIDRequest, stream workload.SpiffeWorkloadAPI_FetchX509SVIDServer) error {
	if err := stream.Send(s.response); err != nil {
		return err
	}
	<-stream.Context().Done()
	return stream.Context().Err()
}

type testSPIREAPIServer struct {
	entryv1.UnimplementedEntryServer
}

func (testSPIREAPIServer) ListEntries(context.Context, *entryv1.ListEntriesRequest) (*entryv1.ListEntriesResponse, error) {
	return &entryv1.ListEntriesResponse{}, nil
}

func startWorkloadAPIServer(t *testing.T, caCert, clientCert *x509.Certificate, clientKey ed25519.PrivateKey, clientID spiffeid.ID) string {
	t.Helper()
	listener, err := net.Listen("unix", t.TempDir()+"/workload-api.sock")
	require.NoError(t, err)

	clientKeyDER, err := x509.MarshalPKCS8PrivateKey(clientKey)
	require.NoError(t, err)
	server := grpc.NewServer()
	workload.RegisterSpiffeWorkloadAPIServer(server, testWorkloadAPIServer{response: &workload.X509SVIDResponse{
		Svids: []*workload.X509SVID{{
			SpiffeId:    clientID.String(),
			X509Svid:    clientCert.Raw,
			X509SvidKey: clientKeyDER,
			Bundle:      caCert.Raw,
		}},
	}})
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
	})
	return "unix://" + listener.Addr().String()
}

func startSPIREAPIServer(t *testing.T, caCert, serverCert *x509.Certificate, serverKey ed25519.PrivateKey) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(caCert)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{serverCert.Raw}, PrivateKey: serverKey}},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
		MinVersion:   tls.VersionTLS12,
	})))
	entryv1.RegisterEntryServer(server, testSPIREAPIServer{})
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
	})
	return listener.Addr().String()
}

func newTestCA(t *testing.T) (*x509.Certificate, ed25519.PrivateKey) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)
	return cert, key
}

func newTestSVID(t *testing.T, caCert *x509.Certificate, caKey ed25519.PrivateKey, id spiffeid.ID) (*x509.Certificate, ed25519.PrivateKey) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	uri, err := url.Parse(id.String())
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		URIs:         []*url.URL{uri},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, key.Public(), caKey)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)
	return cert, key
}
