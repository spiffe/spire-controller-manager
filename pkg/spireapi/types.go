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
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	apitypes "github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Entry struct {
	ID            string
	SPIFFEID      spiffeid.ID
	ParentID      spiffeid.ID
	Selectors     []Selector
	X509SVIDTTL   time.Duration
	JWTSVIDTTL    time.Duration
	FederatesWith []spiffeid.TrustDomain
	Admin         bool
	Downstream    bool
	DNSNames      []string
	Hint          string
	StoreSVID     bool
}

type Selector struct {
	Type  string
	Value string
}

type FederationRelationship struct {
	TrustDomain           spiffeid.TrustDomain
	BundleEndpointURL     string
	BundleEndpointProfile BundleEndpointProfile
	TrustDomainBundle     *spiffebundle.Bundle
}

func (fr FederationRelationship) Equal(other FederationRelationship) bool {
	return fr.TrustDomain == other.TrustDomain &&
		fr.BundleEndpointURL == other.BundleEndpointURL &&
		fr.BundleEndpointProfile.Equal(other.BundleEndpointProfile)
}

type BundleEndpointProfile interface {
	Name() string
	Equal(BundleEndpointProfile) bool
	bundleEndpointProfile()
}

type HTTPSWebProfile struct{}

func (HTTPSWebProfile) Name() string {
	return "https_web"
}

func (HTTPSWebProfile) Equal(other BundleEndpointProfile) bool {
	switch other.(type) {
	case HTTPSWebProfile, *HTTPSWebProfile:
		return true
	default:
		return false
	}
}

func (HTTPSWebProfile) bundleEndpointProfile() {}

type HTTPSSPIFFEProfile struct {
	EndpointSPIFFEID spiffeid.ID
}

func (HTTPSSPIFFEProfile) Name() string {
	return "https_spiffe"
}

func (profile HTTPSSPIFFEProfile) Equal(other BundleEndpointProfile) bool {
	switch other := other.(type) {
	case HTTPSSPIFFEProfile:
		return profile.EndpointSPIFFEID == other.EndpointSPIFFEID
	case *HTTPSSPIFFEProfile:
		return profile.EndpointSPIFFEID == other.EndpointSPIFFEID
	default:
		return false
	}
}

func (HTTPSSPIFFEProfile) bundleEndpointProfile() {}

type JWTKey struct {
	KeyID     string
	PublicKey crypto.PublicKey
	ExpiresAt time.Time
}

type Status struct {
	Code    codes.Code
	Message string
}

func (s Status) Err() error {
	return status.Error(s.Code, s.Message)
}

func ValidateBundleEndpointURL(s string) error {
	if s == "" {
		return errors.New("bundle endpoint URL is missing")
	}
	u, err := url.Parse(s)
	switch {
	case err != nil:
		return err
	case u.Scheme != "https":
		return errors.New("scheme must be https")
	case u.Host == "":
		return errors.New("host must be specified")
	case u.User != nil:
		return errors.New("cannot contain userinfo")
	}
	return nil
}

func entryToAPI(in Entry) *apitypes.Entry {
	return &apitypes.Entry{
		Id:            in.ID,
		SpiffeId:      spiffeIDToAPI(in.SPIFFEID),
		ParentId:      spiffeIDToAPI(in.ParentID),
		Selectors:     selectorsToAPI(in.Selectors),
		X509SvidTtl:   int32(in.X509SVIDTTL / time.Second),
		JwtSvidTtl:    int32(in.JWTSVIDTTL / time.Second),
		FederatesWith: trustDomainsToAPI(in.FederatesWith),
		Admin:         in.Admin,
		DnsNames:      in.DNSNames,
		Downstream:    in.Downstream,
		Hint:          in.Hint,
		StoreSvid:     in.StoreSVID,
	}
}

func entriesToAPI(ins []Entry) []*apitypes.Entry {
	var outs []*apitypes.Entry
	if ins != nil {
		outs = make([]*apitypes.Entry, 0, len(ins))
		for _, in := range ins {
			outs = append(outs, entryToAPI(in))
		}
	}
	return outs
}

func entryFromAPI(in *apitypes.Entry) (Entry, error) {
	if in == nil {
		return Entry{}, errors.New("entry is nil")
	}
	spiffeID, err := spiffeIDFromAPI(in.SpiffeId)
	if err != nil {
		return Entry{}, fmt.Errorf("invalid SPIFFE ID field: %w", err)
	}

	parentID, err := spiffeIDFromAPI(in.ParentId)
	if err != nil {
		return Entry{}, fmt.Errorf("invalid parent ID field: %w", err)
	}

	selectors, err := selectorsFromAPI(in.Selectors)
	if err != nil {
		return Entry{}, fmt.Errorf("invalid selectors field: %w", err)
	}

	federatesWith, err := trustDomainsFromAPI(in.FederatesWith)
	if err != nil {
		return Entry{}, fmt.Errorf("invalid federatesWith field: %w", err)
	}

	return Entry{
		ID:            in.Id,
		SPIFFEID:      spiffeID,
		ParentID:      parentID,
		Selectors:     selectors,
		X509SVIDTTL:   time.Duration(in.X509SvidTtl) * time.Second,
		JWTSVIDTTL:    time.Duration(in.JwtSvidTtl) * time.Second,
		FederatesWith: federatesWith,
		Admin:         in.Admin,
		DNSNames:      in.DnsNames,
		Downstream:    in.Downstream,
		Hint:          in.Hint,
		StoreSVID:     in.StoreSvid,
	}, nil
}

func entriesFromAPI(ins []*apitypes.Entry) ([]Entry, error) {
	var outs []Entry
	if ins != nil {
		outs = make([]Entry, 0, len(ins))
		for _, in := range ins {
			out, err := entryFromAPI(in)
			if err != nil {
				return nil, err
			}
			outs = append(outs, out)
		}
	}
	return outs, nil
}

func spiffeIDToAPI(in spiffeid.ID) *apitypes.SPIFFEID {
	return &apitypes.SPIFFEID{
		TrustDomain: in.TrustDomain().Name(),
		Path:        in.Path(),
	}
}

func spiffeIDFromAPI(in *apitypes.SPIFFEID) (spiffeid.ID, error) {
	if in == nil {
		return spiffeid.ID{}, errors.New("SPIFFE ID is nil")
	}
	td, err := spiffeid.TrustDomainFromString(in.TrustDomain)
	if err != nil {
		return spiffeid.ID{}, err
	}
	return spiffeid.FromPath(td, in.Path)
}

func selectorToAPI(in Selector) *apitypes.Selector {
	return &apitypes.Selector{
		Type:  in.Type,
		Value: in.Value,
	}
}

func selectorsToAPI(ins []Selector) []*apitypes.Selector {
	var outs []*apitypes.Selector
	if ins != nil {
		outs = make([]*apitypes.Selector, 0, len(ins))
		for _, in := range ins {
			outs = append(outs, selectorToAPI(in))
		}
	}
	return outs
}

func selectorFromAPI(in *apitypes.Selector) (Selector, error) {
	switch {
	case in.Type == "":
		return Selector{}, fmt.Errorf("selector type is empty")
	case in.Value == "":
		return Selector{}, fmt.Errorf("selector value is empty")
	case strings.Contains(in.Type, ":"):
		return Selector{}, fmt.Errorf("selector type cannot contain a colon")
	}
	return Selector{
		Type:  in.Type,
		Value: in.Value,
	}, nil
}

func selectorsFromAPI(ins []*apitypes.Selector) ([]Selector, error) {
	var outs []Selector
	if ins != nil {
		outs = make([]Selector, 0, len(ins))
		for _, in := range ins {
			out, err := selectorFromAPI(in)
			if err != nil {
				return nil, err
			}
			outs = append(outs, out)
		}
	}
	return outs, nil
}

func federationRelationshipsToAPI(ins []FederationRelationship) ([]*apitypes.FederationRelationship, error) {
	var outs []*apitypes.FederationRelationship
	if ins != nil {
		outs = make([]*apitypes.FederationRelationship, 0, len(ins))
		for _, in := range ins {
			out, err := federationRelationshipToAPI(in)
			if err != nil {
				return nil, err
			}
			outs = append(outs, out)
		}
	}
	return outs, nil
}

func federationRelationshipToAPI(in FederationRelationship) (*apitypes.FederationRelationship, error) {
	if in.TrustDomain.IsZero() {
		return nil, errors.New("trust domain is missing")
	}
	if err := ValidateBundleEndpointURL(in.BundleEndpointURL); err != nil {
		return nil, fmt.Errorf("invalid bundle endpoint URL: %w", err)
	}

	out := &apitypes.FederationRelationship{
		TrustDomain:       in.TrustDomain.Name(),
		BundleEndpointUrl: in.BundleEndpointURL,
	}

	if in.TrustDomainBundle != nil {
		trustDomainBundle, err := bundleToAPI(in.TrustDomainBundle)
		if err != nil {
			return nil, fmt.Errorf("invalid trust domain bundle: %w", err)
		}
		out.TrustDomainBundle = trustDomainBundle
	}

	switch profile := in.BundleEndpointProfile.(type) {
	case HTTPSWebProfile:
		out.BundleEndpointProfile = &apitypes.FederationRelationship_HttpsWeb{
			HttpsWeb: &apitypes.HTTPSWebProfile{},
		}
	case HTTPSSPIFFEProfile:
		out.BundleEndpointProfile = &apitypes.FederationRelationship_HttpsSpiffe{
			HttpsSpiffe: &apitypes.HTTPSSPIFFEProfile{
				EndpointSpiffeId: profile.EndpointSPIFFEID.String(),
			},
		}
	default:
		return nil, fmt.Errorf("unrecognized bundle endpoint profile type %T", profile)
	}
	return out, nil
}

func federationRelationshipsFromAPI(ins []*apitypes.FederationRelationship) ([]FederationRelationship, error) {
	var outs []FederationRelationship
	if ins != nil {
		outs = make([]FederationRelationship, 0, len(ins))
		for _, in := range ins {
			out, err := federationRelationshipFromAPI(in)
			if err != nil {
				return nil, err
			}
			outs = append(outs, out)
		}
	}
	return outs, nil
}

func federationRelationshipFromAPI(in *apitypes.FederationRelationship) (FederationRelationship, error) {
	trustDomain, err := spiffeid.TrustDomainFromString(in.TrustDomain)
	if err != nil {
		return FederationRelationship{}, fmt.Errorf("invalid trust domain: %w", err)
	}

	if err := ValidateBundleEndpointURL(in.BundleEndpointUrl); err != nil {
		return FederationRelationship{}, fmt.Errorf("invalid bundle endpoint URL: %w", err)
	}

	var trustDomainBundle *spiffebundle.Bundle
	if in.TrustDomainBundle != nil {
		trustDomainBundle, err = bundleFromAPI(in.TrustDomainBundle)
		if err != nil {
			return FederationRelationship{}, fmt.Errorf("invalid trust domain bundle: %w", err)
		}
	}

	var bundleEndpointProfile BundleEndpointProfile
	switch profile := in.BundleEndpointProfile.(type) {
	case *apitypes.FederationRelationship_HttpsWeb:
		if profile.HttpsWeb == nil {
			return FederationRelationship{}, errors.New("https_web profile is missing data")
		}
		bundleEndpointProfile = HTTPSWebProfile{}
	case *apitypes.FederationRelationship_HttpsSpiffe:
		if profile.HttpsSpiffe == nil {
			return FederationRelationship{}, errors.New("https_spiffe profile is missing data")
		}
		endpointSPIFFEID, err := spiffeid.FromString(profile.HttpsSpiffe.EndpointSpiffeId)
		if err != nil {
			return FederationRelationship{}, fmt.Errorf("invalid endpoint SPIFFE ID: %w", err)
		}
		bundleEndpointProfile = HTTPSSPIFFEProfile{
			EndpointSPIFFEID: endpointSPIFFEID,
		}
	case nil:
		return FederationRelationship{}, errors.New("bundle endpoint profile is missing")
	default:
		return FederationRelationship{}, fmt.Errorf("unrecognized bundle endpoint profile type: %T", in.BundleEndpointProfile)
	}

	return FederationRelationship{
		TrustDomain:           trustDomain,
		BundleEndpointURL:     in.BundleEndpointUrl,
		BundleEndpointProfile: bundleEndpointProfile,
		TrustDomainBundle:     trustDomainBundle,
	}, nil
}

func trustDomainsToAPI(ins []spiffeid.TrustDomain) []string {
	var outs []string
	if ins != nil {
		outs = make([]string, 0, len(ins))
		for _, in := range ins {
			outs = append(outs, in.Name())
		}
	}
	return outs
}

func trustDomainsFromAPI(ins []string) ([]spiffeid.TrustDomain, error) {
	var outs []spiffeid.TrustDomain
	if ins != nil {
		outs = make([]spiffeid.TrustDomain, 0, len(ins))
		for _, in := range ins {
			out, err := spiffeid.TrustDomainFromString(in)
			if err != nil {
				return nil, fmt.Errorf("invalid trust domain: %w", err)
			}
			outs = append(outs, out)
		}
	}
	return outs, nil
}

func bundleToAPI(in *spiffebundle.Bundle) (*apitypes.Bundle, error) {
	trustDomain := in.TrustDomain().Name()
	if trustDomain == "" {
		return nil, errors.New("trust domain is missing")
	}
	x509Authorities, err := x509AuthoritiesToAPI(in.X509Authorities())
	if err != nil {
		return nil, err
	}
	jwtAuthorities, err := jwtAuthoritiesToAPI(in.JWTAuthorities())
	if err != nil {
		return nil, err
	}
	sequenceNumber, _ := in.SequenceNumber()
	refreshHint, _ := in.RefreshHint()
	return &apitypes.Bundle{
		TrustDomain:     trustDomain,
		X509Authorities: x509Authorities,
		JwtAuthorities:  jwtAuthorities,
		SequenceNumber:  sequenceNumber,
		RefreshHint:     int64(refreshHint / time.Second),
	}, nil
}

func bundleFromAPI(in *apitypes.Bundle) (*spiffebundle.Bundle, error) {
	if in == nil {
		return nil, nil
	}

	trustDomain, err := spiffeid.TrustDomainFromString(in.TrustDomain)
	if err != nil {
		return nil, err
	}

	x509Authorities, err := x509AuthoritiesFromAPI(in.X509Authorities)
	if err != nil {
		return nil, err
	}

	jwtAuthorities, err := jwtAuthoritiesFromAPI(in.JwtAuthorities)
	if err != nil {
		return nil, err
	}

	out := spiffebundle.New(trustDomain)
	out.SetX509Authorities(x509Authorities)
	out.SetJWTAuthorities(jwtAuthorities)
	if in.SequenceNumber != 0 {
		out.SetSequenceNumber(in.SequenceNumber)
	}
	if in.RefreshHint != 0 {
		out.SetRefreshHint(time.Duration(in.RefreshHint) * time.Second)
	}
	return out, nil
}

func x509AuthoritiesToAPI(ins []*x509.Certificate) ([]*apitypes.X509Certificate, error) {
	var outs []*apitypes.X509Certificate
	if ins != nil {
		outs = make([]*apitypes.X509Certificate, 0, len(ins))
		for _, in := range ins {
			out, err := x509AuthorityToAPI(in)
			if err != nil {
				return nil, err
			}
			outs = append(outs, out)
		}
	}
	return outs, nil
}

func x509AuthorityToAPI(in *x509.Certificate) (*apitypes.X509Certificate, error) {
	if len(in.Raw) == 0 {
		return nil, errors.New("x509 certificate is missing raw data")
	}
	return &apitypes.X509Certificate{Asn1: in.Raw}, nil
}

func x509AuthoritiesFromAPI(ins []*apitypes.X509Certificate) ([]*x509.Certificate, error) {
	var outs []*x509.Certificate
	if ins != nil {
		outs = make([]*x509.Certificate, 0, len(ins))
		for _, in := range ins {
			out, err := x509AuthorityFromAPI(in)
			if err != nil {
				return nil, err
			}
			outs = append(outs, out)
		}
	}
	return outs, nil
}

func x509AuthorityFromAPI(in *apitypes.X509Certificate) (*x509.Certificate, error) {
	return x509.ParseCertificate(in.Asn1)
}

func jwtAuthoritiesToAPI(ins map[string]crypto.PublicKey) ([]*apitypes.JWTKey, error) {
	var outs []*apitypes.JWTKey
	if ins != nil {
		outs = make([]*apitypes.JWTKey, 0, len(ins))
		for keyID, publicKey := range ins {
			out, err := jwtAuthorityToAPI(keyID, publicKey)
			if err != nil {
				return nil, err
			}
			outs = append(outs, out)
		}
	}
	return outs, nil
}

func jwtAuthorityToAPI(keyID string, publicKey crypto.PublicKey) (*apitypes.JWTKey, error) {
	if keyID == "" {
		return nil, errors.New("key ID is missing")
	}
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	return &apitypes.JWTKey{
		KeyId:     keyID,
		PublicKey: publicKeyBytes,
	}, nil
}

func jwtAuthoritiesFromAPI(ins []*apitypes.JWTKey) (map[string]crypto.PublicKey, error) {
	var outs map[string]crypto.PublicKey
	if ins != nil {
		outs = make(map[string]crypto.PublicKey, len(ins))
		for _, in := range ins {
			keyID, publicKey, err := jwtAuthorityFromAPI(in)
			if err != nil {
				return nil, err
			}
			outs[keyID] = publicKey
		}
	}
	return outs, nil
}

func jwtAuthorityFromAPI(in *apitypes.JWTKey) (string, crypto.PublicKey, error) {
	if in.KeyId == "" {
		return "", nil, errors.New("key ID is missing")
	}
	publicKey, err := x509.ParsePKIXPublicKey(in.PublicKey)
	if err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal public key: %w", err)
	}
	return in.KeyId, publicKey, nil
}

func statusFromAPI(in *apitypes.Status) Status {
	if in == nil {
		return Status{
			Code:    codes.Unknown,
			Message: "status is nil",
		}
	}
	return Status{
		Code:    codes.Code(in.Code),
		Message: in.Message,
	}
}
