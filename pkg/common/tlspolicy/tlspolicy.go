// Package tlspolicy parses TLS profile configuration for terminating endpoints.
package tlspolicy

import (
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	cliflag "k8s.io/component-base/cli/flag"
)

// TLSConfig parses tlsProfile and returns a tls.Config.
// When profile is nil or empty, MinVersion is set to TLS 1.2.
func TLSConfig(profile *spirev1alpha1.TLSProfileConfig) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if profile == nil {
		return cfg, nil
	}
	if profile.MinTLSVersion == "" && len(profile.CipherSuites) == 0 && len(profile.CurvePreferences) == 0 {
		return cfg, nil
	}

	if profile.MinTLSVersion != "" {
		minVersion, err := cliflag.TLSVersion(profile.MinTLSVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid minTLSVersion %q: %w", profile.MinTLSVersion, err)
		}
		cfg.MinVersion = minVersion
	}

	if len(profile.CipherSuites) > 0 {
		cipherSuites, err := cliflag.TLSCipherSuites(profile.CipherSuites)
		if err != nil {
			return nil, fmt.Errorf("invalid cipherSuites: %w", err)
		}
		cfg.CipherSuites = cipherSuites
	}

	if len(profile.CurvePreferences) > 0 {
		curves, err := parseCurvePreferences(profile.CurvePreferences)
		if err != nil {
			return nil, fmt.Errorf("invalid curvePreferences: %w", err)
		}
		cfg.CurvePreferences = curves
	}

	return cfg, nil
}

func parseCurvePreferences(names []string) ([]tls.CurveID, error) {
	curves := make([]tls.CurveID, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		if isDecimal(name) {
			id, err := strconv.ParseUint(name, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("invalid curve ID %q: %w", name, err)
			}
			curves = append(curves, tls.CurveID(id))
			continue
		}

		// commonly configured curves
		switch strings.ToLower(name) {
		case "curvep256", "p-256", "p256", "secp256r1":
			curves = append(curves, tls.CurveP256)
		case "curvep384", "p-384", "p384", "secp384r1":
			curves = append(curves, tls.CurveP384)
		case "curvep521", "p-521", "p521", "secp521r1":
			curves = append(curves, tls.CurveP521)
		case "x25519":
			curves = append(curves, tls.X25519)
		case "x25519mlkem768":
			curves = append(curves, tls.X25519MLKEM768)
		case "secp256r1mlkem768":
			curves = append(curves, tls.SecP256r1MLKEM768)
		case "secp384r1mlkem1024":
			curves = append(curves, tls.SecP384r1MLKEM1024)
		default:
			return nil, fmt.Errorf("unknown curve %q", name)
		}
	}

	return curves, nil
}

func isDecimal(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
