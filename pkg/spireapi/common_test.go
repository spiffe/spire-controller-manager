package spireapi

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"sort"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	ctx = context.Background()

	domain1 = spiffeid.RequireTrustDomainFromString("domain1")
	domain2 = spiffeid.RequireTrustDomainFromString("domain2")
	domain3 = spiffeid.RequireTrustDomainFromString("domain3")

	now = time.Now().UTC().Truncate(time.Second)

	key = decodeKey(`-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgUK/9GrMeJQc8qBZE
usHs5xdZrX2sUHPzT0mlkmf0ltihRANCAAS6qfd5FtzLYW+p7NgjqqJuEAyewtzk
4ypsM7PfePnL+45U+mSSypopiiyXvumOlU3uIHpnVhH+dk26KXGHeh2i
-----END PRIVATE KEY-----`)
	publicKeyBytes, _ = x509.MarshalPKIXPublicKey(key.Public())
)

func startServer(t *testing.T, registerFn func(s *grpc.Server)) grpc.ClientConnInterface {
	s := grpc.NewServer()
	registerFn(s)

	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	go func() { _ = s.Serve(listener) }()
	t.Cleanup(s.GracefulStop)

	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	return conn
}

func listBounds(pageToken string, pageSize int, n int, get func(int) string) (int, int, bool) {
	var start int
	if pageToken != "" {
		start = sort.Search(n, func(i int) bool {
			return get(i) > pageToken
		})
	}
	if pageSize > 0 && start+pageSize < n {
		return start, start + pageSize, true
	}
	return start, n, false
}

func assertErrorIs(tb testing.TB, err error, target error) {
	if !errors.Is(err, target) {
		assert.FailNowf(tb, "error does not match error chain", "expected error %+v; got %+v", target, err)
	}
}

func decodeKey(s string) crypto.Signer {
	block, _ := pem.Decode([]byte(s))
	key, _ := x509.ParsePKCS8PrivateKey(block.Bytes)
	return key.(crypto.Signer)
}

func createCertificate(tmpl, parent *x509.Certificate, pub crypto.PublicKey, priv crypto.Signer) (*x509.Certificate, error) {
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, priv)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDER)
}
