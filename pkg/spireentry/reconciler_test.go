package spireentry

import (
	"regexp"
	"testing"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	"github.com/stretchr/testify/require"
)

func TestMakeEntryKey(t *testing.T) {
	id1 := spiffeid.RequireFromString("spiffe://domain.test/1")
	id2 := spiffeid.RequireFromString("spiffe://domain.test/2")
	sAABB := []spireapi.Selector{{Type: "A", Value: "A"}, {Type: "B", Value: "B"}}
	sBBAA := []spireapi.Selector{{Type: "B", Value: "B"}, {Type: "A", Value: "A"}}
	sAAAC := []spireapi.Selector{{Type: "A", Value: "A"}, {Type: "A", Value: "C"}}

	t.Run("same tuple yields same key", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sAABB}
		require.Equal(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("selector order does not matter", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sBBAA}
		require.Equal(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("parent ID changes key", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB}
		b := spireapi.Entry{ID: "B", ParentID: id2, SPIFFEID: id2, Selectors: sAABB}
		require.NotEqual(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("SPIFFE ID changes key", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id1, Selectors: sAABB}
		require.NotEqual(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("Selectors change key", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sAAAC}
		require.NotEqual(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("TTL has no impact", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, X509SVIDTTL: 1}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, X509SVIDTTL: 2}
		require.Equal(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("FederatesWith has no impact", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, FederatesWith: []spiffeid.TrustDomain{spiffeid.RequireTrustDomainFromString("domaina")}}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, FederatesWith: []spiffeid.TrustDomain{spiffeid.RequireTrustDomainFromString("domainb")}}
		require.Equal(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("Admin has no impact", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, Admin: false}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, Admin: true}
		require.Equal(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("Downstream has no impact", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, Downstream: false}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, Downstream: true}
		require.Equal(t, makeEntryKey(a), makeEntryKey(b))
	})

	t.Run("DNSNames have no impact", func(t *testing.T) {
		a := spireapi.Entry{ID: "A", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, DNSNames: []string{"A"}}
		b := spireapi.Entry{ID: "B", ParentID: id1, SPIFFEID: id2, Selectors: sAABB, DNSNames: []string{"B"}}
		require.Equal(t, makeEntryKey(a), makeEntryKey(b))
	})
}

func TestIsNamespaceIgnored(t *testing.T) {
	tests := []struct {
		ignoredNamespaces []string
		namespace         string
		expected          bool
	}{
		{[]string{"s([a-z]+)re", "default"}, "spire", true},
		{[]string{"s([a-z]+)re", "default"}, "default", true},
		{[]string{"s([a-z]+)re", "default"}, "spiffe", false},
		{[]string{"s([a-z]+)re", "default"}, "kubernetes", false},
	}

	for _, test := range tests {
		var regexIgnoredNamespaces []*regexp.Regexp
		for _, ignoredNamespace := range test.ignoredNamespaces {
			regex, err := regexp.Compile(ignoredNamespace)

			if err == nil {
				regexIgnoredNamespaces = append(regexIgnoredNamespaces, regex)
			}
		}

		actual := isNamespaceIgnored(regexIgnoredNamespaces, test.namespace)
		require.Equalf(t, test.expected, actual, "isNamespaceIgnored(%s, %s): expected %t, actual %t",
			test.ignoredNamespaces, test.namespace, test.expected, actual)
	}
}
