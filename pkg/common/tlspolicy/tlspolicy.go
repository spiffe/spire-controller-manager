// Package tlspolicy parses TLS configuration for terminating endpoints.
package tlspolicy

import (
	"crypto/tls"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	cliflag "k8s.io/component-base/cli/flag"
)

// TLSConfig parses tlsConfig and returns a tls.Config.
// When tlsConfig is nil or empty, MinVersion is set to TLS 1.2.
func TLSConfig(tlsCfg *spirev1alpha1.TLSConfig, log logr.Logger) (*tls.Config, error) {
	if log.GetSink() == nil {
		// Production call site will have a logger, so we don't need to discard logs.
		log = logr.Discard()
	}

	// Default to TLS 1.2 if no configuration is provided.
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if tlsCfg == nil {
		return cfg, nil
	}
	if tlsCfg.MinTLSVersion == "" && len(tlsCfg.CipherSuites) == 0 && len(tlsCfg.CurvePreferences) == 0 {
		return cfg, nil
	}

	if tlsCfg.MinTLSVersion != "" {
		minVersion, err := cliflag.TLSVersion(tlsCfg.MinTLSVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid minTLSVersion %q: %w", tlsCfg.MinTLSVersion, err)
		}
		if minVersion < tls.VersionTLS12 {
			return nil, fmt.Errorf("minTLSVersion %q must be VersionTLS12 or higher", tlsCfg.MinTLSVersion)
		}
		cfg.MinVersion = minVersion
	}

	if len(tlsCfg.CipherSuites) > 0 {
		if cfg.MinVersion >= tls.VersionTLS13 {
			log.Info("cipherSuites is ignored because minTLSVersion is VersionTLS13 or higher; Go negotiates TLS 1.3 cipher suites automatically")
		} else {
			cipherSuites, err := parseCipherSuites(tlsCfg.CipherSuites, log)
			if err != nil {
				return nil, fmt.Errorf("invalid cipherSuites: %w", err)
			}
			if len(cipherSuites) == 0 {
				log.Info("no supported cipherSuites remain after filtering; Go TLS defaults will be used")
			} else {
				cfg.CipherSuites = cipherSuites
			}
		}
	}

	if len(tlsCfg.CurvePreferences) > 0 {
		curves, err := curvePreferences(tlsCfg.CurvePreferences, cfg.MinVersion, log)
		if err != nil {
			return nil, fmt.Errorf("invalid curvePreferences: %w", err)
		}
		if len(curves) == 0 {
			log.Info("no supported curvePreferences remain after filtering; Go TLS defaults will be used")
		} else {
			cfg.CurvePreferences = curves
		}
	}

	return cfg, nil
}

func parseCipherSuites(names []string, log logr.Logger) ([]uint16, error) {
	insecureCiphers := cliflag.InsecureTLSCiphers()
	secureCiphers := make([]string, 0, len(names))
	for _, name := range names {
		if _, ok := insecureCiphers[name]; ok {
			log.Info("insecure cipher suite filtered out", "cipherSuite", name)
			continue
		}
		secureCiphers = append(secureCiphers, name)
	}
	return cliflag.TLSCipherSuites(secureCiphers)
}

var wellknownCurveAliases = map[string]int32{
	"secp256r1":          int32(tls.CurveP256),
	"secp384r1":          int32(tls.CurveP384),
	"secp521r1":          int32(tls.CurveP521),
	"x25519":             int32(tls.X25519),
	"x25519mlkem768":     int32(tls.X25519MLKEM768),
	"secp256r1mlkem768":  int32(tls.SecP256r1MLKEM768),
	"secp384r1mlkem1024": int32(tls.SecP384r1MLKEM1024),
}

func curvePreferences(names []string, minVersion uint16, log logr.Logger) ([]tls.CurveID, error) {
	givenCurveIDs := make([]int32, 0, len(names))

	// Convert given names to curve IDs.
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		id, err := curveIDFromName(name)
		if err != nil {
			return nil, err
		}
		givenCurveIDs = append(givenCurveIDs, id)
	}

	// Convert curveIDs to tls.CurveID. This will validate the curve IDs and return an error if any are invalid.
	curves, err := cliflag.TLSCurvePreferences(givenCurveIDs)
	if err != nil {
		return nil, err
	}

	// If minVersion is TLS 1.3 or higher, or no curves are given, return the curves
	if minVersion >= tls.VersionTLS13 || len(curves) == 0 {
		return curves, nil
	}

	// If minVersion is below TLS 1.3, check if the curves include at least one classical curve supported by TLS<1.3
	hasClassical := false
	for _, curve := range curves {
		if isTLS13OnlyCurve(curve) {
			log.Info("TLS 1.2 does not support curve", "curve", curve.String())
			continue
		}
		hasClassical = true
	}
	if !hasClassical {
		return nil, errors.New("curvePreferences must include at least one classical curve when minTLSVersion is below VersionTLS13")
	}

	return curves, nil
}

func curveIDFromName(name string) (int32, error) {
	if id, ok := wellknownCurveAliases[strings.ToLower(name)]; ok {
		return id, nil
	}

	id, err := strconv.ParseInt(name, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("unknown curve %q", name)
	}

	return int32(id), nil
}

func isTLS13OnlyCurve(curve tls.CurveID) bool {
	switch curve {
	case tls.X25519MLKEM768, tls.SecP256r1MLKEM768, tls.SecP384r1MLKEM1024:
		return true
	default:
		return false
	}
}
