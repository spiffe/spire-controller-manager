package tlspolicy_test

import (
	"crypto/tls"
	"fmt"
	"testing"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/spiffe/spire-controller-manager/pkg/common/tlspolicy"
	"github.com/stretchr/testify/require"
)

func TestTLSConfigNil(t *testing.T) {
	cfg, err := tlspolicy.TLSConfig(nil)
	require.NoError(t, err)
	require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	require.Empty(t, cfg.CipherSuites)
	require.Empty(t, cfg.CurvePreferences)
}

func TestTLSConfigEmpty(t *testing.T) {
	cfg, err := tlspolicy.TLSConfig(&spirev1alpha1.TLSProfileConfig{})
	require.NoError(t, err)
	require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	require.Empty(t, cfg.CipherSuites)
	require.Empty(t, cfg.CurvePreferences)
}

func TestTLSConfigProfile(t *testing.T) {
	cfg, err := tlspolicy.TLSConfig(&spirev1alpha1.TLSProfileConfig{
		MinTLSVersion: "VersionTLS13",
		CipherSuites:  []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
		CurvePreferences: []string{
			"X25519MLKEM768",
			"secp256r1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
	require.NotEmpty(t, cfg.CipherSuites)
	require.Equal(t, []tls.CurveID{tls.X25519MLKEM768, tls.CurveP256}, cfg.CurvePreferences)
}

func TestTLSConfigMinTLSVersionOnly(t *testing.T) {
	cfg, err := tlspolicy.TLSConfig(&spirev1alpha1.TLSProfileConfig{
		MinTLSVersion: "VersionTLS12",
	})
	require.NoError(t, err)
	require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	require.Empty(t, cfg.CipherSuites)
	require.Empty(t, cfg.CurvePreferences)
}

func TestTLSConfigCipherSuitesOnly(t *testing.T) {
	cfg, err := tlspolicy.TLSConfig(&spirev1alpha1.TLSProfileConfig{
		CipherSuites: []string{
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		},
	})
	require.NoError(t, err)
	require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	require.Len(t, cfg.CipherSuites, 2)
	require.Empty(t, cfg.CurvePreferences)
}

func TestTLSConfigInvalidMinTLSVersion(t *testing.T) {
	_, err := tlspolicy.TLSConfig(&spirev1alpha1.TLSProfileConfig{
		MinTLSVersion: "VersionTLS99",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid minTLSVersion")
}

func TestTLSConfigInvalidCipherSuite(t *testing.T) {
	_, err := tlspolicy.TLSConfig(&spirev1alpha1.TLSProfileConfig{
		CipherSuites: []string{"TLS_NOT_A_CIPHER"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid cipherSuites")
}

func TestTLSConfigInvalidCurve(t *testing.T) {
	_, err := tlspolicy.TLSConfig(&spirev1alpha1.TLSProfileConfig{
		CurvePreferences: []string{"unknown-curve"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid curvePreferences")
}

func TestTLSConfigCurvePreferences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  tls.CurveID
	}{
		{"curvep256", "CurveP256", tls.CurveP256},
		{"p256", "P256", tls.CurveP256},
		{"p-256", "P-256", tls.CurveP256},
		{"secp256r1", "secp256r1", tls.CurveP256},
		{"curvep384", "CurveP384", tls.CurveP384},
		{"p384", "P384", tls.CurveP384},
		{"p-384", "P-384", tls.CurveP384},
		{"secp384r1", "secp384r1", tls.CurveP384},
		{"curvep521", "CurveP521", tls.CurveP521},
		{"p521", "P521", tls.CurveP521},
		{"p-521", "P-521", tls.CurveP521},
		{"secp521r1", "secp521r1", tls.CurveP521},
		{"x25519", "X25519", tls.X25519},
		{"x25519mlkem768", "X25519MLKEM768", tls.X25519MLKEM768},
		{"secp256r1mlkem768", "SecP256r1MLKEM768", tls.SecP256r1MLKEM768},
		{"secp384r1mlkem1024", "SecP384r1MLKEM1024", tls.SecP384r1MLKEM1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := tlspolicy.TLSConfig(&spirev1alpha1.TLSProfileConfig{
				CurvePreferences: []string{tt.input},
			})
			require.NoError(t, err)
			require.Equal(t, []tls.CurveID{tt.want}, cfg.CurvePreferences)
		})
	}
}

func TestTLSConfigCurvePreferencesDecimalID(t *testing.T) {
	cfg, err := tlspolicy.TLSConfig(&spirev1alpha1.TLSProfileConfig{
		CurvePreferences: []string{fmt.Sprintf("%d", tls.X25519)},
	})
	require.NoError(t, err)
	require.Equal(t, []tls.CurveID{tls.X25519}, cfg.CurvePreferences)
}

func TestTLSConfigCurvePreferencesSkipsEmptyAndTrimsWhitespace(t *testing.T) {
	cfg, err := tlspolicy.TLSConfig(&spirev1alpha1.TLSProfileConfig{
		CurvePreferences: []string{"", "  X25519  ", " secp256r1"},
	})
	require.NoError(t, err)
	require.Equal(t, []tls.CurveID{tls.X25519, tls.CurveP256}, cfg.CurvePreferences)
}

func TestTLSConfigCurvePreferencesInvalidDecimalID(t *testing.T) {
	_, err := tlspolicy.TLSConfig(&spirev1alpha1.TLSProfileConfig{
		CurvePreferences: []string{"999999"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid curve ID")
}

func TestTLSConfigCurvePreferencesPreservesOrder(t *testing.T) {
	cfg, err := tlspolicy.TLSConfig(&spirev1alpha1.TLSProfileConfig{
		CurvePreferences: []string{
			"X25519MLKEM768",
			"X25519",
			"secp256r1",
			"secp384r1",
		},
	})
	require.NoError(t, err)
	require.Equal(t, []tls.CurveID{
		tls.X25519MLKEM768,
		tls.X25519,
		tls.CurveP256,
		tls.CurveP384,
	}, cfg.CurvePreferences)
}

func TestTLSConfigCipherSuitesPreservesOrder(t *testing.T) {
	first := "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"
	second := "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"

	cfg, err := tlspolicy.TLSConfig(&spirev1alpha1.TLSProfileConfig{
		CipherSuites: []string{first, second},
	})
	require.NoError(t, err)
	require.Equal(t, []uint16{
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	}, cfg.CipherSuites)
}
