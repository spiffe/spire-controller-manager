package tlspolicy_test

import (
	"crypto/tls"
	"testing"

	"github.com/spiffe/spire-controller-manager/pkg/common/tlspolicy"
	"github.com/stretchr/testify/require"
)

func TestApplyPolicy(t *testing.T) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.X25519MLKEM768, tls.CurveP256,
		},
	}
	err := tlspolicy.ApplyPolicy(tlsConfig, tlspolicy.Policy{
		RequirePQKEM: true,
	})
	require.NoError(t, err)

	require.Equal(t, []tls.CurveID{tls.X25519MLKEM768}, tlsConfig.CurvePreferences)
	require.Equal(t, uint16(tls.VersionTLS13), tlsConfig.MinVersion)
}

func TestApplyPolicyDisabled(t *testing.T) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
		},
	}
	err := tlspolicy.ApplyPolicy(tlsConfig, tlspolicy.Policy{})
	require.NoError(t, err)

	require.Equal(t, []tls.CurveID{tls.CurveP256}, tlsConfig.CurvePreferences)
	require.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
}
