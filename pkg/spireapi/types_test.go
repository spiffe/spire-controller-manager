package spireapi

import (
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	apitypes "github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

var (
	entry = Entry{
		ID:            "ID",
		SPIFFEID:      spiffeid.RequireFromString("spiffe://domain1.test/workload"),
		ParentID:      spiffeid.RequireFromString("spiffe://domain1.test/parent"),
		Selectors:     []Selector{{Type: "Type", Value: "Value"}},
		X509SVIDTTL:   time.Minute,
		FederatesWith: []spiffeid.TrustDomain{spiffeid.RequireTrustDomainFromString("domain2.test")},
		Admin:         true,
		Downstream:    true,
		DNSNames:      []string{"dnsname"},
		StoreSVID:     true,
	}

	apiEntry = &apitypes.Entry{
		Id: "ID",
		SpiffeId: &apitypes.SPIFFEID{
			TrustDomain: "domain1.test",
			Path:        "/workload",
		},
		ParentId: &apitypes.SPIFFEID{
			TrustDomain: "domain1.test",
			Path:        "/parent",
		},
		Selectors:     []*apitypes.Selector{{Type: "Type", Value: "Value"}},
		X509SvidTtl:   60,
		FederatesWith: []string{"domain2.test"},
		Admin:         true,
		Downstream:    true,
		DnsNames:      []string{"dnsname"},
		StoreSvid:     true,
	}
)

func TestFederationRelationshipEqual(t *testing.T) {
	tdA := spiffeid.RequireTrustDomainFromString("a")
	tdB := spiffeid.RequireTrustDomainFromString("b")

	bundleEndpointURLA := "a"
	bundleEndpointURLB := "b"

	bundleEndpointProfileA := HTTPSWebProfile{}
	bundleEndpointProfileB := HTTPSSPIFFEProfile{EndpointSPIFFEID: spiffeid.RequireFromString("spiffe://b/endpoint")}
	bundleEndpointProfileC := HTTPSSPIFFEProfile{EndpointSPIFFEID: spiffeid.RequireFromString("spiffe://c/endpoint")}

	bundleA := spiffebundle.New(tdA)
	bundleB := spiffebundle.New(tdB)

	base := FederationRelationship{
		TrustDomain:           tdA,
		BundleEndpointURL:     bundleEndpointURLA,
		BundleEndpointProfile: bundleEndpointProfileA,
		TrustDomainBundle:     bundleA,
	}

	assertEqual := func(t *testing.T, mutate func(*FederationRelationship)) {
		compareTo := base
		mutate(&compareTo)
		assert.True(t, base.Equal(compareTo))
	}
	assertNotEqual := func(t *testing.T, mutate func(*FederationRelationship)) {
		compareTo := base
		mutate(&compareTo)
		assert.False(t, base.Equal(compareTo))
	}

	assertEqual(t, func(compareTo *FederationRelationship) {})
	assertNotEqual(t, func(compareTo *FederationRelationship) {
		compareTo.TrustDomain = tdB
	})
	assertNotEqual(t, func(compareTo *FederationRelationship) {
		compareTo.BundleEndpointURL = bundleEndpointURLB
	})
	assertNotEqual(t, func(compareTo *FederationRelationship) {
		compareTo.BundleEndpointProfile = bundleEndpointProfileB
	})
	assertNotEqual(t, func(compareTo *FederationRelationship) {
		compareTo.BundleEndpointProfile = bundleEndpointProfileC
	})

	// Bundles aren't considered for equality purposes.
	assertEqual(t, func(compareTo *FederationRelationship) {
		compareTo.TrustDomainBundle = bundleB
	})
}

func TestProfileNames(t *testing.T) {
	assert.Equal(t, "https_web", (HTTPSWebProfile{}).Name())
	assert.Equal(t, "https_spiffe", (HTTPSSPIFFEProfile{}).Name())
}

func TestHTTPSWebProfileEquality(t *testing.T) {
	assert.True(t, (HTTPSWebProfile{}).Equal(HTTPSWebProfile{}))
	assert.False(t, (HTTPSWebProfile{}).Equal(HTTPSSPIFFEProfile{}))
}

func TestHTTPSSPIFFEProfileEquality(t *testing.T) {
	idA := HTTPSSPIFFEProfile{EndpointSPIFFEID: spiffeid.RequireFromString("spiffe://a/endpoint")}
	idACopy := HTTPSSPIFFEProfile{EndpointSPIFFEID: spiffeid.RequireFromString("spiffe://a/endpoint")}
	idB := HTTPSSPIFFEProfile{EndpointSPIFFEID: spiffeid.RequireFromString("spiffe://b/endpoint")}
	idBCopy := HTTPSSPIFFEProfile{EndpointSPIFFEID: spiffeid.RequireFromString("spiffe://b/endpoint")}

	assert.True(t, idA.Equal(idACopy))
	assert.False(t, idA.Equal(idB))
	assert.True(t, idB.Equal(idBCopy))
	assert.False(t, idB.Equal(idA))

	// With pointer
	assert.True(t, idA.Equal(&idA))
	assert.False(t, idA.Equal(&idB))
	assert.False(t, idB.Equal(&idA))
	assert.True(t, idB.Equal(&idB))

	assert.False(t, idB.Equal(HTTPSWebProfile{}))
}

func TestStatusErr(t *testing.T) {
	err := status.Error(codes.InvalidArgument, "oh no")
	assert.True(t, errors.Is((Status{Code: codes.InvalidArgument, Message: "oh no"}).Err(), err))
}

func TestEntryToAPI(t *testing.T) {
	assertProtoEqual(t, apiEntry, entryToAPI(entry))
}

func TestEntriesToAPI(t *testing.T) {
	assert.Empty(t, entriesToAPI(nil))

	actual := entriesToAPI([]Entry{entry})
	require.Len(t, actual, 1)
	assertProtoEqual(t, apiEntry, actual[0])
}

func TestEntryFromAPI(t *testing.T) {
	for _, tc := range []struct {
		desc        string
		makeEntry   func(*apitypes.Entry) *apitypes.Entry
		expectEntry Entry
		expectErr   string
	}{
		{
			desc: "nil entry",
			makeEntry: func(base *apitypes.Entry) *apitypes.Entry {
				return nil
			},
			expectErr: "entry is nil",
		},
		{
			desc: "nil SPIFFE ID",
			makeEntry: func(base *apitypes.Entry) *apitypes.Entry {
				base.SpiffeId = nil
				return base
			},
			expectErr: "invalid SPIFFE ID field: SPIFFE ID is nil",
		},
		{
			desc: "invalid SPIFFE ID",
			makeEntry: func(base *apitypes.Entry) *apitypes.Entry {
				base.SpiffeId = &apitypes.SPIFFEID{}
				return base
			},
			expectErr: "invalid SPIFFE ID field: trust domain is missing",
		},
		{
			desc: "nil parent ID",
			makeEntry: func(base *apitypes.Entry) *apitypes.Entry {
				base.ParentId = nil
				return base
			},
			expectErr: "invalid parent ID field: SPIFFE ID is nil",
		},
		{
			desc: "invalid parent ID",
			makeEntry: func(base *apitypes.Entry) *apitypes.Entry {
				base.ParentId = &apitypes.SPIFFEID{}
				return base
			},
			expectErr: "invalid parent ID field: trust domain is missing",
		},
		{
			desc: "missing selector type",
			makeEntry: func(base *apitypes.Entry) *apitypes.Entry {
				base.Selectors[0].Type = ""
				return base
			},
			expectErr: "invalid selectors field: selector type is empty",
		},
		{
			desc: "invalid selector type",
			makeEntry: func(base *apitypes.Entry) *apitypes.Entry {
				base.Selectors[0].Type = "bad:type"
				return base
			},
			expectErr: "invalid selectors field: selector type cannot contain a colon",
		},
		{
			desc: "missing selector value",
			makeEntry: func(base *apitypes.Entry) *apitypes.Entry {
				base.Selectors[0].Value = ""
				return base
			},
			expectErr: "invalid selectors field: selector value is empty",
		},
		{
			desc: "invalid federatesWith value",
			makeEntry: func(base *apitypes.Entry) *apitypes.Entry {
				base.FederatesWith = []string{"INVALID"}
				return base
			},
			expectErr: "invalid federatesWith field: invalid trust domain: trust domain characters are limited to lowercase letters, numbers, dots, dashes, and underscores",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			entry, err := entryFromAPI(tc.makeEntry(proto.Clone(apiEntry).(*apitypes.Entry)))
			if tc.expectErr != "" {
				assert.EqualError(t, err, tc.expectErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectEntry, entry)
		})
	}
}

func TestEntriesFromAPI(t *testing.T) {
	actual, err := entriesFromAPI(nil)
	assert.NoError(t, err)
	assert.Empty(t, actual)

	apiEntries := []*apitypes.Entry{
		proto.Clone(apiEntry).(*apitypes.Entry),
	}

	actual, err = entriesFromAPI(apiEntries)
	assert.NoError(t, err)
	assert.Equal(t, entry, actual[0])

	apiEntries[0].SpiffeId = nil

	actual, err = entriesFromAPI(apiEntries)
	assert.EqualError(t, err, "invalid SPIFFE ID field: SPIFFE ID is nil")
	assert.Empty(t, actual)
}

func TestFederationRelationshipToAPI(t *testing.T) {
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	x509Authority, err := createCertificate(tmpl, tmpl, key.Public(), key)
	require.NoError(t, err)

	td := spiffeid.RequireTrustDomainFromString("domain.test")
	bundleEndpointURL := "https://domain.test/bundle"
	endpointSPIFFEID := spiffeid.RequireFromPath(td, "/bundle/endpoint")

	bundle := spiffebundle.New(td)
	bundle.SetX509Authorities([]*x509.Certificate{x509Authority})
	bundle.SetJWTAuthorities(map[string]crypto.PublicKey{"KEYID": key.Public()})

	apiBundle := &apitypes.Bundle{
		TrustDomain:     "domain.test",
		X509Authorities: []*apitypes.X509Certificate{{Asn1: x509Authority.Raw}},
		JwtAuthorities:  []*apitypes.JWTKey{{KeyId: "KEYID", PublicKey: publicKeyBytes}},
	}
	for _, tc := range []struct {
		desc      string
		fr        FederationRelationship
		expectFR  *apitypes.FederationRelationship
		expectErr string
	}{
		{
			desc: "missing trust domain",
			fr: FederationRelationship{
				BundleEndpointURL:     bundleEndpointURL,
				BundleEndpointProfile: HTTPSWebProfile{},
			},
			expectErr: "trust domain is missing",
		},
		{
			desc: "invalid bundle endpoint URL",
			fr: FederationRelationship{
				TrustDomain:           td,
				BundleEndpointProfile: HTTPSWebProfile{},
			},
			expectErr: "invalid bundle endpoint URL: bundle endpoint URL is missing",
		},
		{
			desc: "trust domain bundle missing trust domain",
			fr: FederationRelationship{
				TrustDomain:           td,
				BundleEndpointURL:     bundleEndpointURL,
				BundleEndpointProfile: HTTPSWebProfile{},
				TrustDomainBundle:     &spiffebundle.Bundle{},
			},
			expectErr: "invalid trust domain bundle: trust domain is missing",
		},
		{
			desc: "trust domain bundle has invalid X.509 authority",
			fr: FederationRelationship{
				TrustDomain:           td,
				BundleEndpointURL:     bundleEndpointURL,
				BundleEndpointProfile: HTTPSWebProfile{},
				TrustDomainBundle:     spiffebundle.FromX509Authorities(td, []*x509.Certificate{{}}),
			},
			expectErr: "invalid trust domain bundle: x509 certificate is missing raw data",
		},
		{
			desc: "trust domain bundle has invalid JWT authority key id",
			fr: FederationRelationship{
				TrustDomain:           td,
				BundleEndpointURL:     bundleEndpointURL,
				BundleEndpointProfile: HTTPSWebProfile{},
				TrustDomainBundle:     spiffebundle.FromJWTAuthorities(td, map[string]crypto.PublicKey{"": key.Public()}),
			},
			expectErr: "invalid trust domain bundle: key ID is missing",
		},
		{
			desc: "trust domain bundle has invalid JWT authority public key",
			fr: FederationRelationship{
				TrustDomain:           td,
				BundleEndpointURL:     bundleEndpointURL,
				BundleEndpointProfile: HTTPSWebProfile{},
				TrustDomainBundle:     spiffebundle.FromJWTAuthorities(td, map[string]crypto.PublicKey{"KEYID": nil}),
			},
			expectErr: "invalid trust domain bundle: failed to marshal public key: x509: unsupported public key type: <nil>",
		},
		{
			desc: "unrecognized profile type",
			fr: FederationRelationship{
				TrustDomain:       td,
				BundleEndpointURL: bundleEndpointURL,
			},
			expectErr: "unrecognized bundle endpoint profile type <nil>",
		},
		{
			desc: "success with https_web",
			fr: FederationRelationship{
				TrustDomain:           td,
				BundleEndpointURL:     bundleEndpointURL,
				BundleEndpointProfile: HTTPSWebProfile{},
				TrustDomainBundle:     bundle,
			},
			expectFR: &apitypes.FederationRelationship{
				TrustDomain:       td.Name(),
				BundleEndpointUrl: bundleEndpointURL,
				BundleEndpointProfile: &apitypes.FederationRelationship_HttpsWeb{
					HttpsWeb: &apitypes.HTTPSWebProfile{},
				},
				TrustDomainBundle: apiBundle,
			},
		},
		{
			desc: "success with https_spiffe",
			fr: FederationRelationship{
				TrustDomain:       td,
				BundleEndpointURL: bundleEndpointURL,
				BundleEndpointProfile: HTTPSSPIFFEProfile{
					EndpointSPIFFEID: endpointSPIFFEID,
				},
				TrustDomainBundle: bundle,
			},
			expectFR: &apitypes.FederationRelationship{
				TrustDomain:       td.Name(),
				BundleEndpointUrl: bundleEndpointURL,
				BundleEndpointProfile: &apitypes.FederationRelationship_HttpsSpiffe{
					HttpsSpiffe: &apitypes.HTTPSSPIFFEProfile{
						EndpointSpiffeId: endpointSPIFFEID.String(),
					},
				},
				TrustDomainBundle: apiBundle,
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fr, err := federationRelationshipToAPI(tc.fr)
			if tc.expectErr != "" {
				assert.EqualError(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
			assertProtoEqual(t, tc.expectFR, fr)
		})
	}
}

func TestFederationRelationshipFromAPI(t *testing.T) {
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	x509Authority, err := createCertificate(tmpl, tmpl, key.Public(), key)
	require.NoError(t, err)

	td := spiffeid.RequireTrustDomainFromString("domain.test")
	bundleEndpointURL := "https://domain.test/bundle"
	endpointSPIFFEID := spiffeid.RequireFromPath(td, "/bundle/endpoint")
	httpsWebProfile := &apitypes.FederationRelationship_HttpsWeb{HttpsWeb: &apitypes.HTTPSWebProfile{}}
	httpsSPIFFEProfile := &apitypes.FederationRelationship_HttpsSpiffe{HttpsSpiffe: &apitypes.HTTPSSPIFFEProfile{
		EndpointSpiffeId: endpointSPIFFEID.String(),
	}}

	bundle := spiffebundle.New(td)
	bundle.SetX509Authorities([]*x509.Certificate{x509Authority})
	bundle.SetJWTAuthorities(map[string]crypto.PublicKey{"KEYID": key.Public()})
	bundle.SetSequenceNumber(1234)
	bundle.SetRefreshHint(time.Hour)

	apiBundle := &apitypes.Bundle{
		TrustDomain:     "domain.test",
		X509Authorities: []*apitypes.X509Certificate{{Asn1: x509Authority.Raw}},
		JwtAuthorities:  []*apitypes.JWTKey{{KeyId: "KEYID", PublicKey: publicKeyBytes}},
		SequenceNumber:  1234,
		RefreshHint:     3600,
	}
	for _, tc := range []struct {
		desc      string
		fr        *apitypes.FederationRelationship
		expectFR  FederationRelationship
		expectErr string
	}{
		{
			desc: "missing trust domain",
			fr: &apitypes.FederationRelationship{
				BundleEndpointUrl:     bundleEndpointURL,
				BundleEndpointProfile: httpsWebProfile,
			},
			expectErr: "invalid trust domain: trust domain is missing",
		},
		{
			desc: "invalid bundle endpoint URL",
			fr: &apitypes.FederationRelationship{
				TrustDomain:           td.Name(),
				BundleEndpointProfile: httpsWebProfile,
			},
			expectErr: "invalid bundle endpoint URL: bundle endpoint URL is missing",
		},
		{
			desc: "trust domain bundle missing trust domain",
			fr: &apitypes.FederationRelationship{
				TrustDomain:       td.Name(),
				BundleEndpointUrl: bundleEndpointURL,
				TrustDomainBundle: &apitypes.Bundle{},
			},
			expectErr: "invalid trust domain bundle: trust domain is missing",
		},
		{
			desc: "trust domain bundle has invalid X.509 authority",
			fr: &apitypes.FederationRelationship{
				TrustDomain:       td.Name(),
				BundleEndpointUrl: bundleEndpointURL,
				TrustDomainBundle: &apitypes.Bundle{TrustDomain: td.Name(), X509Authorities: []*apitypes.X509Certificate{{}}},
			},
			expectErr: "invalid trust domain bundle: x509: malformed certificate",
		},
		{
			desc: "trust domain bundle has invalid JWT authority key id",
			fr: &apitypes.FederationRelationship{
				TrustDomain:       td.Name(),
				BundleEndpointUrl: bundleEndpointURL,
				TrustDomainBundle: &apitypes.Bundle{TrustDomain: td.Name(), JwtAuthorities: []*apitypes.JWTKey{{PublicKey: publicKeyBytes}}},
			},
			expectErr: "invalid trust domain bundle: key ID is missing",
		},
		{
			desc: "trust domain bundle has invalid JWT authority public key",
			fr: &apitypes.FederationRelationship{
				TrustDomain:       td.Name(),
				BundleEndpointUrl: bundleEndpointURL,
				TrustDomainBundle: &apitypes.Bundle{TrustDomain: td.Name(), JwtAuthorities: []*apitypes.JWTKey{{KeyId: "KEYID"}}},
			},
			expectErr: "invalid trust domain bundle: failed to unmarshal public key: asn1: syntax error: sequence truncated",
		},
		{
			desc: "unrecognized profile type",
			fr: &apitypes.FederationRelationship{
				TrustDomain:       td.Name(),
				BundleEndpointUrl: bundleEndpointURL,
			},
			expectErr: "bundle endpoint profile is missing",
		},
		{
			desc: "https_web profile is missing data",
			fr: &apitypes.FederationRelationship{
				TrustDomain:           td.Name(),
				BundleEndpointUrl:     bundleEndpointURL,
				BundleEndpointProfile: &apitypes.FederationRelationship_HttpsWeb{},
			},
			expectErr: "https_web profile is missing data",
		},
		{
			desc: "https_spiffe profile is missing data",
			fr: &apitypes.FederationRelationship{
				TrustDomain:           td.Name(),
				BundleEndpointUrl:     bundleEndpointURL,
				BundleEndpointProfile: &apitypes.FederationRelationship_HttpsSpiffe{},
			},
			expectErr: "https_spiffe profile is missing data",
		},
		{
			desc: "https_spiffe profile is has an invalid endpoint SPIFFE ID",
			fr: &apitypes.FederationRelationship{
				TrustDomain:       td.Name(),
				BundleEndpointUrl: bundleEndpointURL,
				BundleEndpointProfile: &apitypes.FederationRelationship_HttpsSpiffe{
					HttpsSpiffe: &apitypes.HTTPSSPIFFEProfile{},
				},
			},
			expectErr: "invalid endpoint SPIFFE ID: cannot be empty",
		},
		{
			desc: "success with https_web",
			fr: &apitypes.FederationRelationship{
				TrustDomain:           td.Name(),
				BundleEndpointUrl:     bundleEndpointURL,
				BundleEndpointProfile: httpsWebProfile,
				TrustDomainBundle:     apiBundle,
			},
			expectFR: FederationRelationship{
				TrustDomain:           td,
				BundleEndpointURL:     bundleEndpointURL,
				BundleEndpointProfile: HTTPSWebProfile{},
				TrustDomainBundle:     bundle,
			},
		},
		{
			desc: "success with https_spiffe",
			fr: &apitypes.FederationRelationship{
				TrustDomain:           td.Name(),
				BundleEndpointUrl:     bundleEndpointURL,
				BundleEndpointProfile: httpsSPIFFEProfile,
				TrustDomainBundle:     apiBundle,
			},
			expectFR: FederationRelationship{
				TrustDomain:       td,
				BundleEndpointURL: bundleEndpointURL,
				BundleEndpointProfile: HTTPSSPIFFEProfile{
					EndpointSPIFFEID: endpointSPIFFEID,
				},
				TrustDomainBundle: bundle,
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fr, err := federationRelationshipFromAPI(tc.fr)
			if tc.expectErr != "" {
				assert.EqualError(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectFR, fr)
		})
	}
}

func TestValidateBundleEndpointURL(t *testing.T) {
	assert.EqualError(t, ValidateBundleEndpointURL(""), "bundle endpoint URL is missing")
	assert.EqualError(t, ValidateBundleEndpointURL("http://domain.test"), "scheme must be https")
	assert.EqualError(t, ValidateBundleEndpointURL("https:///path"), "host must be specified")
	assert.EqualError(t, ValidateBundleEndpointURL("https://joe@domain.test"), "cannot contain userinfo")
	assert.NoError(t, ValidateBundleEndpointURL("https://domain.test"))
	assert.NoError(t, ValidateBundleEndpointURL("https://domain.test:443"))
}

func assertProtoEqual(t *testing.T, expected, actual proto.Message) {
	if diff := cmp.Diff(expected, actual, protocmp.Transform()); diff != "" {
		assert.Fail(t, fmt.Sprintf("protobuf is not equal: %s", diff))
	}
}
