package spireentry

import (
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

func TestFilterJoinTokenEntries(t *testing.T) {
	id1 := spiffeid.RequireFromString("spiffe://domain.test/1")
	id2 := spiffeid.RequireFromString("spiffe://domain.test/2")
	id3 := spiffeid.RequireFromString("spiffe://domain.test/3")
	idJoinToken := spiffeid.RequireFromString("spiffe://domain.test/spire/agent/join_token/717290d1-6e81-40cc-b9c4-1416f8c30cfd")
	s1 := []spireapi.Selector{{Type: "A", Value: "A"}, {Type: "B", Value: "B"}}
	s2 := []spireapi.Selector{{Type: "B", Value: "B"}, {Type: "A", Value: "A"}}
	sJoinToken := []spireapi.Selector{{Type: "spiffe_id", Value: "A"}}

	testCases := []struct {
		name     string
		entries  []spireapi.Entry
		expected []spireapi.Entry
	}{
		{
			name: "no join token entries",
			entries: []spireapi.Entry{
				{ID: "1", ParentID: id1, SPIFFEID: id2, Selectors: s1},
				{ID: "2", ParentID: id1, SPIFFEID: id3, Selectors: s2},
			},
			expected: []spireapi.Entry{
				{ID: "1", ParentID: id1, SPIFFEID: id2, Selectors: s1},
				{ID: "2", ParentID: id1, SPIFFEID: id3, Selectors: s2},
			},
		},
		{
			name: "with join token entries",
			entries: []spireapi.Entry{
				{ID: "1", ParentID: id1, SPIFFEID: id2, Selectors: s1},
				{ID: "2", ParentID: id1, SPIFFEID: id3, Selectors: s2},
				{ID: "3", ParentID: idJoinToken, SPIFFEID: id3, Selectors: sJoinToken},
				{ID: "4", ParentID: idJoinToken, SPIFFEID: id2, Selectors: sJoinToken},
			},
			expected: []spireapi.Entry{
				{ID: "1", ParentID: id1, SPIFFEID: id2, Selectors: s1},
				{ID: "2", ParentID: id1, SPIFFEID: id3, Selectors: s2},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := filterJoinTokenEntries(tc.entries)
			require.Equal(t, tc.expected, actual)
		})
	}
}
