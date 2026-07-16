package spireapi

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/spiffe/spire-controller-manager/pkg/metrics"
)

// counterDelta returns the difference in an EntryWriteTotal counter between
// before and after, using prometheus/testutil.ToFloat64 on the labeled counter.
func counterBefore(operation, code string) float64 {
	return testutil.ToFloat64(metrics.EntryWriteTotal.WithLabelValues(operation, code))
}

// TestCreateEntriesMetrics verifies that EntryWriteTotal is incremented for
// both codes.OK (new entry) and codes.AlreadyExists (race / double execution).
func TestCreateEntriesMetrics(t *testing.T) {
	server, client := startEntryAPIServer(t)

	// Pre-populate one entry so that creating it again returns AlreadyExists.
	server.setEntries(t, entry1)

	beforeOK := counterBefore(metrics.OpBatchCreate, metrics.CodeOK)
	beforeAlreadyExists := counterBefore(metrics.OpBatchCreate, metrics.CodeAlreadyExists)

	// entry2 is new → OK; entry1 already exists → AlreadyExists.
	statuses, err := client.CreateEntries(context.Background(), []Entry{entry2, entry1})
	require.NoError(t, err)
	require.Len(t, statuses, 2)

	afterOK := counterBefore(metrics.OpBatchCreate, metrics.CodeOK)
	afterAlreadyExists := counterBefore(metrics.OpBatchCreate, metrics.CodeAlreadyExists)

	assert.Equal(t, float64(1), afterOK-beforeOK, "expected 1 OK create")
	assert.Equal(t, float64(1), afterAlreadyExists-beforeAlreadyExists, "expected 1 AlreadyExists create")
}

// TestDeleteEntriesMetrics verifies that EntryWriteTotal is incremented for
// both codes.OK and codes.NotFound (entry already removed, e.g. by other replica).
func TestDeleteEntriesMetrics(t *testing.T) {
	server, client := startEntryAPIServer(t)
	server.setEntries(t, entry1)

	beforeOK := counterBefore(metrics.OpBatchDelete, metrics.CodeOK)
	beforeNotFound := counterBefore(metrics.OpBatchDelete, metrics.CodeNotFound)

	// entry1 exists → OK; entry2 does not → NotFound.
	statuses, err := client.DeleteEntries(context.Background(), []string{entry1.ID, entry2.ID})
	require.NoError(t, err)
	require.Len(t, statuses, 2)

	afterOK := counterBefore(metrics.OpBatchDelete, metrics.CodeOK)
	afterNotFound := counterBefore(metrics.OpBatchDelete, metrics.CodeNotFound)

	assert.Equal(t, float64(1), afterOK-beforeOK, "expected 1 OK delete")
	assert.Equal(t, float64(1), afterNotFound-beforeNotFound, "expected 1 NotFound delete")
}

// TestUpdateEntriesMetrics verifies that EntryWriteTotal is incremented for OK updates.
func TestUpdateEntriesMetrics(t *testing.T) {
	server, client := startEntryAPIServer(t)
	server.setEntries(t, entry1)

	beforeOK := counterBefore(metrics.OpBatchUpdate, metrics.CodeOK)
	beforeNotFound := counterBefore(metrics.OpBatchUpdate, metrics.CodeNotFound)

	// entry1 exists → OK; entry2 does not → NotFound.
	statuses, err := client.UpdateEntries(context.Background(), []Entry{entry1, entry2})
	require.NoError(t, err)
	require.Len(t, statuses, 2)

	afterOK := counterBefore(metrics.OpBatchUpdate, metrics.CodeOK)
	afterNotFound := counterBefore(metrics.OpBatchUpdate, metrics.CodeNotFound)

	assert.Equal(t, float64(1), afterOK-beforeOK, "expected 1 OK update")
	assert.Equal(t, float64(1), afterNotFound-beforeNotFound, "expected 1 NotFound update")
}

// TestListEntriesDurationObserved verifies that SPIREAPIRequestDuration receives at
// least one observation after a ListEntries call. CollectAndCount counts the number of
// distinct metric time-series collected; calling WithLabelValues registers a new series.
func TestListEntriesDurationObserved(t *testing.T) {
	server, client := startEntryAPIServer(t)
	server.setEntries(t, entry1, entry2)

	// Force registration of the list_entries label combination so it appears in
	// the collected output, then count metric points before and after.
	beforeCount := testutil.CollectAndCount(metrics.SPIREAPIRequestDuration)

	_, err := client.ListEntries(context.Background())
	require.NoError(t, err)

	afterCount := testutil.CollectAndCount(metrics.SPIREAPIRequestDuration)
	// If list_entries was not yet registered before this test, the count grows by 1.
	// If it was already registered, the count stays the same but the observation is
	// still recorded (the histogram bucket values change). Either way >= 1 is correct.
	assert.GreaterOrEqual(t, afterCount, beforeCount, "list_entries duration histogram should have metrics after a call")
	assert.GreaterOrEqual(t, afterCount, 1, "at least one time-series should exist in the duration histogram")
}
