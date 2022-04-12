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
	TTL           time.Duration
	FederatesWith []spiffeid.TrustDomain
	Admin         bool
	DnsNames      []string
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

func entryToAPI(in Entry) (*apitypes.Entry, error) {
	// TODO: input validation
	spiffeID, err := spiffeIDToAPI(in.SPIFFEID)
	if err != nil {
		return nil, err
	}
	parentID, err := spiffeIDToAPI(in.ParentID)
	if err != nil {
		return nil, err
	}
	selectors, err := selectorsToAPI(in.Selectors)
	if err != nil {
		return nil, err
	}
	federatesWith, err := trustDomainsToAPI(in.FederatesWith)
	if err != nil {
		return nil, err
	}
	return &apitypes.Entry{
		Id:            in.ID,
		SpiffeId:      spiffeID,
		ParentId:      parentID,
		Selectors:     selectors,
		Ttl:           int32(in.TTL / time.Second),
		FederatesWith: federatesWith,
		Admin:         in.Admin,
		DnsNames:      in.DnsNames,
	}, nil
}

func entriesToAPI(ins []Entry) ([]*apitypes.Entry, error) {
	if ins == nil {
		return nil, nil
	}
	outs := make([]*apitypes.Entry, 0, len(ins))
	for _, in := range ins {
		out, err := entryToAPI(in)
		if err != nil {
			return nil, err
		}
		outs = append(outs, out)
	}
	return outs, nil
}

func entryFromAPI(in *apitypes.Entry) (Entry, error) {
	spiffeID, err := spiffeIDFromAPI(in.SpiffeId)
	if err != nil {
		return Entry{}, fmt.Errorf("invalid spiffe_id: %w", err)
	}

	parentID, err := spiffeIDFromAPI(in.ParentId)
	if err != nil {
		return Entry{}, fmt.Errorf("invalid parent_id: %w", err)
	}

	selectors, err := selectorsFromAPI(in.Selectors)
	if err != nil {
		return Entry{}, fmt.Errorf("invalid selectors: %w", err)
	}

	federatesWith, err := trustDomainsFromAPI(in.FederatesWith)
	if err != nil {
		return Entry{}, fmt.Errorf("invalid federates_with: %w", err)
	}

	return Entry{
		ID:            in.Id,
		SPIFFEID:      spiffeID,
		ParentID:      parentID,
		Selectors:     selectors,
		TTL:           time.Duration(in.Ttl) * time.Second,
		FederatesWith: federatesWith,
		Admin:         in.Admin,
		DnsNames:      in.DnsNames,
	}, nil
}

func entriesFromAPI(ins []*apitypes.Entry) ([]Entry, error) {
	outs := make([]Entry, 0, len(ins))
	for _, in := range ins {
		out, err := entryFromAPI(in)
		if err != nil {
			return nil, err
		}
		outs = append(outs, out)
	}
	return outs, nil
}

func spiffeIDToAPI(in spiffeid.ID) (*apitypes.SPIFFEID, error) {
	// TODO: input validation
	return &apitypes.SPIFFEID{
		TrustDomain: in.TrustDomain().String(),
		Path:        in.Path(),
	}, nil
}

func spiffeIDFromAPI(in *apitypes.SPIFFEID) (spiffeid.ID, error) {
	switch {
	case in == nil:
		return spiffeid.ID{}, errors.New("id is nil")
	case in.TrustDomain == "":
		return spiffeid.ID{}, errors.New("id has no trust domain")
	case in.Path != "" && in.Path[0] != '/':
		return spiffeid.ID{}, errors.New("id path is relative")
	}
	td, err := spiffeid.TrustDomainFromString(in.TrustDomain)
	if err != nil {
		return spiffeid.ID{}, err
	}
	return spiffeid.FromPath(td, in.Path)
}

func selectorToAPI(in Selector) (*apitypes.Selector, error) {
	// TODO: input validation
	return &apitypes.Selector{
		Type:  in.Type,
		Value: in.Value,
	}, nil
}

func selectorsToAPI(ins []Selector) ([]*apitypes.Selector, error) {
	if ins == nil {
		return nil, nil
	}
	outs := make([]*apitypes.Selector, 0, len(ins))
	for _, in := range ins {
		out, err := selectorToAPI(in)
		if err != nil {
			return nil, err
		}
		outs = append(outs, out)
	}
	return outs, nil
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
	outs := make([]Selector, 0, len(ins))
	for _, in := range ins {
		out, err := selectorFromAPI(in)
		if err != nil {
			return nil, err
		}
		outs = append(outs, out)
	}
	return outs, nil
}

func federationRelationshipsToAPI(ins []FederationRelationship) ([]*apitypes.FederationRelationship, error) {
	if ins == nil {
		return nil, nil
	}
	outs := make([]*apitypes.FederationRelationship, 0, len(ins))
	for _, in := range ins {
		out, err := federationRelationshipToAPI(in)
		if err != nil {
			return nil, err
		}
		outs = append(outs, out)
	}
	return outs, nil
}

func federationRelationshipToAPI(in FederationRelationship) (*apitypes.FederationRelationship, error) {
	// TODO: input validation
	out := &apitypes.FederationRelationship{
		TrustDomain:       in.TrustDomain.String(),
		BundleEndpointUrl: in.BundleEndpointURL,
	}

	if in.TrustDomainBundle != nil {
		trustDomainBundle, err := bundleToAPI(in.TrustDomainBundle)
		if err != nil {
			return nil, err
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
	}
	return out, nil
}

func federationRelationshipsFromAPI(ins []*apitypes.FederationRelationship) ([]FederationRelationship, error) {
	if ins == nil {
		return nil, nil
	}
	outs := make([]FederationRelationship, 0, len(ins))
	for _, in := range ins {
		out, err := federationRelationshipFromAPI(in)
		if err != nil {
			return nil, err
		}
		outs = append(outs, out)
	}
	return outs, nil
}

func federationRelationshipFromAPI(in *apitypes.FederationRelationship) (FederationRelationship, error) {
	trustDomain, err := spiffeid.TrustDomainFromString(in.TrustDomain)
	if err != nil {
		return FederationRelationship{}, fmt.Errorf("invalid trust domain: %w", err)
	}

	if _, err := url.Parse(in.BundleEndpointUrl); err != nil {
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
			return FederationRelationship{}, errors.New("https_web profile missing data")
		}
		bundleEndpointProfile = HTTPSWebProfile{}
	case *apitypes.FederationRelationship_HttpsSpiffe:
		if profile.HttpsSpiffe == nil {
			return FederationRelationship{}, errors.New("https_spiffe profile missing data")
		}
		endpointSPIFFEID, err := spiffeid.FromString(profile.HttpsSpiffe.EndpointSpiffeId)
		if err != nil {
			return FederationRelationship{}, fmt.Errorf("invalid endpoint SPIFFE ID: %w", err)
		}
		bundleEndpointProfile = HTTPSSPIFFEProfile{
			EndpointSPIFFEID: endpointSPIFFEID,
		}
	case nil:
		return FederationRelationship{}, errors.New("bundle endpoint profile missing")
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

func trustDomainsToAPI(ins []spiffeid.TrustDomain) ([]string, error) {
	if ins == nil {
		return nil, nil
	}
	// TODO: input validation
	outs := make([]string, 0, len(ins))
	for _, in := range ins {
		outs = append(outs, in.String())
	}
	return outs, nil
}

func trustDomainsFromAPI(ins []string) ([]spiffeid.TrustDomain, error) {
	outs := make([]spiffeid.TrustDomain, 0, len(ins))
	for _, in := range ins {
		out, err := spiffeid.TrustDomainFromString(in)
		if err != nil {
			return nil, fmt.Errorf("invalid trust domain: %w", err)
		}
		outs = append(outs, out)
	}
	return outs, nil
}

func bundleToAPI(in *spiffebundle.Bundle) (*apitypes.Bundle, error) {
	if in == nil {
		return nil, nil
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
		TrustDomain:     in.TrustDomain().String(),
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
	if ins == nil {
		return nil, nil
	}
	outs := make([]*apitypes.X509Certificate, 0, len(ins))
	for _, in := range ins {
		out, err := x509AuthorityToAPI(in)
		if err != nil {
			return nil, err
		}
		outs = append(outs, out)
	}
	return outs, nil
}

func x509AuthorityToAPI(in *x509.Certificate) (*apitypes.X509Certificate, error) {
	// TODO: input validation
	return &apitypes.X509Certificate{Asn1: in.Raw}, nil
}

func x509AuthoritiesFromAPI(ins []*apitypes.X509Certificate) ([]*x509.Certificate, error) {
	if ins == nil {
		return nil, nil
	}
	outs := make([]*x509.Certificate, 0, len(ins))
	for _, in := range ins {
		out, err := x509AuthorityFromAPI(in)
		if err != nil {
			return nil, err
		}
		outs = append(outs, out)
	}
	return outs, nil
}

func x509AuthorityFromAPI(in *apitypes.X509Certificate) (*x509.Certificate, error) {
	return x509.ParseCertificate(in.Asn1)
}

func jwtAuthoritiesToAPI(ins map[string]crypto.PublicKey) ([]*apitypes.JWTKey, error) {
	if ins == nil {
		return nil, nil
	}
	outs := make([]*apitypes.JWTKey, 0, len(ins))
	for keyID, publicKey := range ins {
		out, err := jwtAuthorityToAPI(keyID, publicKey)
		if err != nil {
			return nil, err
		}
		outs = append(outs, out)
	}
	return outs, nil
}

func jwtAuthorityToAPI(keyID string, publicKey crypto.PublicKey) (*apitypes.JWTKey, error) {
	if keyID == "" {
		return nil, errors.New("key ID is missing")
	}
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %v", err)
	}
	return &apitypes.JWTKey{
		KeyId:     keyID,
		PublicKey: publicKeyBytes,
	}, nil
}

func jwtAuthoritiesFromAPI(ins []*apitypes.JWTKey) (map[string]crypto.PublicKey, error) {
	if ins == nil {
		return nil, nil
	}
	outs := make(map[string]crypto.PublicKey, len(ins))
	for _, in := range ins {
		keyID, publicKey, err := jwtAuthorityFromAPI(in)
		if err != nil {
			return nil, err
		}
		outs[keyID] = publicKey
	}
	return outs, nil
}

func jwtAuthorityFromAPI(in *apitypes.JWTKey) (string, crypto.PublicKey, error) {
	if in.KeyId == "" {
		return "", nil, errors.New("key ID is missing")
	}
	publicKey, err := x509.ParsePKIXPublicKey(in.PublicKey)
	if err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal public key: %v", err)
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
