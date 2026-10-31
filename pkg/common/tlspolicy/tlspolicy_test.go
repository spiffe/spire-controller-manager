package tlspolicy_test

import (
	"crypto/tls"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/spiffe/spire-controller-manager/pkg/common/tlspolicy"
	"github.com/stretchr/testify/require"
)

func TestTLSConfigValid(t *testing.T) {
	ecdheRSA := tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
	ecdheECDSA := tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384

	tests := []struct {
		name       string
		in         *spirev1alpha1.TLSConfig
		wantMin    uint16
		wantCipher []uint16
		wantCurves []tls.CurveID
	}{
		{
			name:    "nil config defaults to TLS 1.2",
			in:      nil,
			wantMin: tls.VersionTLS12,
		},
		{
			name:    "empty config defaults to TLS 1.2",
			in:      &spirev1alpha1.TLSConfig{},
			wantMin: tls.VersionTLS12,
		},
		{
			name: "minTLSVersion VersionTLS12 only",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion: "VersionTLS12",
			},
			wantMin: tls.VersionTLS12,
		},
		{
			name: "minTLSVersion VersionTLS13 only",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion: "VersionTLS13",
			},
			wantMin: tls.VersionTLS13,
		},
		{
			name: "cipherSuites only",
			in: &spirev1alpha1.TLSConfig{
				CipherSuites: []string{
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
					"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
				},
			},
			wantMin:    tls.VersionTLS12,
			wantCipher: []uint16{ecdheRSA, ecdheECDSA},
		},
		{
			name: "insecure cipherSuites filtered to Go defaults",
			in: &spirev1alpha1.TLSConfig{
				CipherSuites: []string{"TLS_ECDHE_RSA_WITH_RC4_128_SHA"},
			},
			wantMin:    tls.VersionTLS12,
			wantCipher: nil,
		},
		{
			name: "mixed secure and insecure cipherSuites",
			in: &spirev1alpha1.TLSConfig{
				CipherSuites: []string{
					"TLS_ECDHE_RSA_WITH_RC4_128_SHA",
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				},
			},
			wantMin:    tls.VersionTLS12,
			wantCipher: []uint16{ecdheRSA},
		},
		{
			name: "cipherSuites ignored when minTLSVersion is VersionTLS13",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion: "VersionTLS13",
				CipherSuites:  []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
			},
			wantMin:    tls.VersionTLS13,
			wantCipher: nil,
		},
		{
			name: "classical curvePreferences only with default min TLS 1.2",
			in: &spirev1alpha1.TLSConfig{
				CipherSuites:     []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
				CurvePreferences: []string{"X25519", "secp256r1"},
			},
			wantMin:    tls.VersionTLS12,
			wantCipher: []uint16{ecdheRSA},
			wantCurves: []tls.CurveID{tls.X25519, tls.CurveP256},
		},
		{
			name: "classical curvePreferences with explicit TLS 1.2",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion:    "VersionTLS12",
				CurvePreferences: []string{"secp384r1", "secp521r1"},
			},
			wantMin:    tls.VersionTLS12,
			wantCurves: []tls.CurveID{tls.CurveP384, tls.CurveP521},
		},
		{
			name: "hybrid and classical curvePreferences with TLS 1.2 min",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion: "VersionTLS12",
				CurvePreferences: []string{
					"X25519MLKEM768",
					"X25519",
					"secp256r1",
				},
			},
			wantMin: tls.VersionTLS12,
			wantCurves: []tls.CurveID{
				tls.X25519MLKEM768,
				tls.X25519,
				tls.CurveP256,
			},
		},
		{
			name: "hybrid curvePreferences only with TLS 1.3 min",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion: "VersionTLS13",
				CurvePreferences: []string{
					"X25519MLKEM768",
					"SecP256r1MLKEM768",
					"SecP384r1MLKEM1024",
				},
			},
			wantMin: tls.VersionTLS13,
			wantCurves: []tls.CurveID{
				tls.X25519MLKEM768,
				tls.SecP256r1MLKEM768,
				tls.SecP384r1MLKEM1024,
			},
		},
		{
			name: "TLS 1.3 with classical and hybrid curves",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion: "VersionTLS13",
				CurvePreferences: []string{
					"X25519MLKEM768",
					"secp256r1",
				},
			},
			wantMin: tls.VersionTLS13,
			wantCurves: []tls.CurveID{
				tls.X25519MLKEM768,
				tls.CurveP256,
			},
		},
		{
			name: "curvePreferences preserve order",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion: "VersionTLS13",
				CurvePreferences: []string{
					"X25519MLKEM768",
					"X25519",
					"secp256r1",
					"secp384r1",
				},
			},
			wantMin: tls.VersionTLS13,
			wantCurves: []tls.CurveID{
				tls.X25519MLKEM768,
				tls.X25519,
				tls.CurveP256,
				tls.CurveP384,
			},
		},
		{
			name: "curvePreferences decimal ID",
			in: &spirev1alpha1.TLSConfig{
				CurvePreferences: []string{fmt.Sprintf("%d", tls.X25519)},
			},
			wantMin:    tls.VersionTLS12,
			wantCurves: []tls.CurveID{tls.X25519},
		},
		{
			name: "curvePreferences mixed decimal and named",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion: "VersionTLS13",
				CurvePreferences: []string{
					fmt.Sprintf("%d", tls.X25519MLKEM768),
					"secp256r1",
					fmt.Sprintf("%d", tls.X25519),
				},
			},
			wantMin: tls.VersionTLS13,
			wantCurves: []tls.CurveID{
				tls.X25519MLKEM768,
				tls.CurveP256,
				tls.X25519,
			},
		},
		{
			name: "curvePreferences skip empty and trim whitespace",
			in: &spirev1alpha1.TLSConfig{
				CurvePreferences: []string{"", "  X25519  ", " secp256r1"},
			},
			wantMin:    tls.VersionTLS12,
			wantCurves: []tls.CurveID{tls.X25519, tls.CurveP256},
		},
		{
			name: "curvePreferences whitespace only uses Go defaults",
			in: &spirev1alpha1.TLSConfig{
				CurvePreferences: []string{"", "   "},
			},
			wantMin: tls.VersionTLS12,
		},
		{
			name: "curve name aliases are case insensitive",
			in: &spirev1alpha1.TLSConfig{
				CurvePreferences: []string{"SECP256R1", "x25519mlkem768"},
				MinTLSVersion:    "VersionTLS13",
			},
			wantMin: tls.VersionTLS13,
			wantCurves: []tls.CurveID{
				tls.CurveP256,
				tls.X25519MLKEM768,
			},
		},
		{
			name: "full TLS 1.2 profile with ciphers and curves",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion: "VersionTLS12",
				CipherSuites: []string{
					"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
					"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
				},
				CurvePreferences: []string{"X25519", "secp256r1"},
			},
			wantMin:    tls.VersionTLS12,
			wantCipher: []uint16{ecdheRSA, ecdheECDSA},
			wantCurves: []tls.CurveID{tls.X25519, tls.CurveP256},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := tlspolicy.TLSConfig(tt.in, logr.Discard())
			require.NoError(t, err)
			require.Equal(t, tt.wantMin, cfg.MinVersion)
			require.Equal(t, tt.wantCipher, cfg.CipherSuites)
			require.Equal(t, tt.wantCurves, cfg.CurvePreferences)
		})
	}
}

func TestTLSConfigInvalid(t *testing.T) {
	tests := []struct {
		name    string
		in      *spirev1alpha1.TLSConfig
		errPart string
	}{
		{
			name: "unknown minTLSVersion",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion: "VersionTLS99",
			},
			errPart: "invalid minTLSVersion",
		},
		{
			name: "minTLSVersion VersionTLS10 rejected",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion: "VersionTLS10",
			},
			errPart: "must be VersionTLS12 or higher",
		},
		{
			name: "minTLSVersion VersionTLS11 rejected",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion: "VersionTLS11",
			},
			errPart: "must be VersionTLS12 or higher",
		},
		{
			name: "unknown cipherSuite",
			in: &spirev1alpha1.TLSConfig{
				CipherSuites: []string{"TLS_NOT_A_CIPHER"},
			},
			errPart: "invalid cipherSuites",
		},
		{
			name: "unknown curve name",
			in: &spirev1alpha1.TLSConfig{
				CurvePreferences: []string{"unknown-curve"},
			},
			errPart: "invalid curvePreferences",
		},
		{
			name: "hybrid-only curves with explicit TLS 1.2 min",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion:    "VersionTLS12",
				CurvePreferences: []string{"X25519MLKEM768"},
			},
			errPart: "at least one classical curve",
		},
		{
			name: "hybrid-only curves with default TLS 1.2 min",
			in: &spirev1alpha1.TLSConfig{
				CurvePreferences: []string{"X25519MLKEM768"},
			},
			errPart: "at least one classical curve",
		},
		{
			name: "multiple hybrid-only curves with TLS 1.2 min",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion: "VersionTLS12",
				CurvePreferences: []string{
					"X25519MLKEM768",
					"SecP256r1MLKEM768",
					"SecP384r1MLKEM1024",
				},
			},
			errPart: "at least one classical curve",
		},
		{
			name: "decimal curve ID out of range",
			in: &spirev1alpha1.TLSConfig{
				CurvePreferences: []string{"999999"},
			},
			errPart: "out of range",
		},
		{
			name: "unsupported decimal curve ID",
			in: &spirev1alpha1.TLSConfig{
				CurvePreferences: []string{"9999"},
			},
			errPart: "not supported",
		},
		{
			name: "duplicate curvePreferences",
			in: &spirev1alpha1.TLSConfig{
				CurvePreferences: []string{"X25519", "X25519"},
			},
			errPart: "duplicate curve preference",
		},
		{
			name: "invalid minTLSVersion prevents later field parsing",
			in: &spirev1alpha1.TLSConfig{
				MinTLSVersion:    "VersionTLS99",
				CurvePreferences: []string{"X25519"},
			},
			errPart: "invalid minTLSVersion",
		},
		{
			name: "invalid cipherSuite prevents curve parsing",
			in: &spirev1alpha1.TLSConfig{
				CipherSuites:     []string{"TLS_NOT_A_CIPHER"},
				CurvePreferences: []string{"X25519"},
			},
			errPart: "invalid cipherSuites",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tlspolicy.TLSConfig(tt.in, logr.Discard())
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errPart)
		})
	}
}

func TestTLSConfigCurveNameAliases(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want tls.CurveID
	}{
		{"secp256r1", "secp256r1", tls.CurveP256},
		{"secp384r1", "secp384r1", tls.CurveP384},
		{"secp521r1", "secp521r1", tls.CurveP521},
		{"x25519", "X25519", tls.X25519},
		{"x25519mlkem768", "X25519MLKEM768", tls.X25519MLKEM768},
		{"secp256r1mlkem768", "SecP256r1MLKEM768", tls.SecP256r1MLKEM768},
		{"secp384r1mlkem1024", "SecP384r1MLKEM1024", tls.SecP384r1MLKEM1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minTLSVersion := ""
			if tt.want == tls.X25519MLKEM768 || tt.want == tls.SecP256r1MLKEM768 || tt.want == tls.SecP384r1MLKEM1024 {
				minTLSVersion = "VersionTLS13"
			}

			cfg, err := tlspolicy.TLSConfig(&spirev1alpha1.TLSConfig{
				MinTLSVersion:    minTLSVersion,
				CurvePreferences: []string{tt.in},
			}, logr.Discard())
			require.NoError(t, err)
			require.Equal(t, []tls.CurveID{tt.want}, cfg.CurvePreferences)
		})
	}
}
