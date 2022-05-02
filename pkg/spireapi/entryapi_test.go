package spireapi

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	entryv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/entry/v1"
	apitypes "github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func init() {
	entryCreateBatchSize = 2
	entryUpdateBatchSize = 2
	entryDeleteBatchSize = 2
	entryListPageSize = 2
}

var (
	entry1ID = "E1"
	entry1   = Entry{
		ID:        entry1ID,
		ParentID:  spiffeid.RequireFromString("spiffe://domain.test/parent"),
		SPIFFEID:  spiffeid.RequireFromString("spiffe://domain.test/workload1"),
		Selectors: []Selector{{Type: "T1", Value: "V1"}},
	}
	entry2ID = "E2"
	entry2   = Entry{
		ID:        entry2ID,
		ParentID:  spiffeid.RequireFromString("spiffe://domain.test/parent"),
		SPIFFEID:  spiffeid.RequireFromString("spiffe://domain.test/workload2"),
		Selectors: []Selector{{Type: "T2", Value: "V2"}},
	}
	entry3ID = "E3"
	entry3   = Entry{
		ID:        entry3ID,
		ParentID:  spiffeid.RequireFromString("spiffe://domain.test/parent"),
		SPIFFEID:  spiffeid.RequireFromString("spiffe://domain.test/workload3"),
		Selectors: []Selector{{Type: "T3", Value: "V3"}},
	}
)

func TestEntryAPIListEntries(t *testing.T) {
	server, client := startEntryAPIServer(t)

	for _, tc := range []struct {
		desc          string
		expectEntries []Entry
		expectErr     error
	}{
		{
			desc:      "error",
			expectErr: status.Error(codes.Internal, "oh no"),
		},
		{
			desc:          "empty",
			expectEntries: nil,
		},
		{
			desc:          "less than a page",
			expectEntries: []Entry{entry1},
		},
		{
			desc:          "exactly a page",
			expectEntries: []Entry{entry1, entry2},
		},
		{
			desc:          "more than a page",
			expectEntries: []Entry{entry1, entry2, entry3},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			server.listEntriesErr = tc.expectErr
			server.setEntries(t, tc.expectEntries...)
			actualEntries, err := client.ListEntries(ctx)
			if tc.expectErr != nil {
				assertErrorIs(t, err, tc.expectErr)
				assert.Empty(t, actualEntries)
				return
			}
			assert.NoError(t, err)
			assert.ElementsMatch(t, tc.expectEntries, actualEntries)
		})
	}
}

func TestCreateEntries(t *testing.T) {
	server, client := startEntryAPIServer(t)

	ok := Status{Code: codes.OK}

	for _, tc := range []struct {
		desc          string
		withEntries   []Entry
		createEntries []Entry
		expectEntries []Entry
		expectStatus  []Status
		expectErr     error
	}{
		{
			desc:          "empty",
			expectEntries: nil,
			expectStatus:  []Status{},
		},
		{
			desc:          "RPC error",
			createEntries: []Entry{entry1},
			expectErr:     status.Error(codes.Internal, "oh no"),
		},
		{
			desc:          "already exists",
			withEntries:   []Entry{entry1},
			createEntries: []Entry{entry1},
			expectEntries: []Entry{entry1},
			expectStatus:  []Status{{Code: codes.AlreadyExists, Message: `entry "E1" already exists`}},
		},
		{
			desc:          "less than a batch",
			createEntries: []Entry{entry1},
			expectEntries: []Entry{entry1},
			expectStatus:  []Status{ok},
		},
		{
			desc:          "exactly a batch",
			createEntries: []Entry{entry1, entry2},
			expectEntries: []Entry{entry1, entry2},
			expectStatus:  []Status{ok, ok},
		},
		{
			desc:          "more than a batch",
			createEntries: []Entry{entry1, entry2, entry3},
			expectEntries: []Entry{entry1, entry2, entry3},
			expectStatus:  []Status{ok, ok, ok},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			server.setEntries(t, tc.withEntries...)
			server.batchCreateEntriesErr = tc.expectErr
			actualStatus, err := client.CreateEntries(ctx, tc.createEntries)
			if tc.expectErr != nil {
				assertErrorIs(t, err, tc.expectErr)
				assert.Empty(t, actualStatus)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectStatus, actualStatus)
			assert.ElementsMatch(t, tc.expectEntries, server.getEntries(t))
		})
	}
}

func TestUpdateEntries(t *testing.T) {
	server, client := startEntryAPIServer(t)

	ok := Status{Code: codes.OK}

	dupWithTTL := func(entry Entry, ttl time.Duration) Entry {
		entry.TTL = ttl
		return entry
	}

	entry1Old := dupWithTTL(entry1, 1*time.Second)
	entry2Old := dupWithTTL(entry2, 2*time.Second)
	entry3Old := dupWithTTL(entry3, 3*time.Second)

	for _, tc := range []struct {
		desc          string
		withEntries   []Entry
		updateEntries []Entry
		expectEntries []Entry
		expectStatus  []Status
		expectErr     error
	}{
		{
			desc:          "empty",
			expectEntries: nil,
			expectStatus:  []Status{},
		},
		{
			desc:          "RPC error",
			updateEntries: []Entry{entry1},
			expectErr:     status.Error(codes.Internal, "oh no"),
		},
		{
			desc:          "not found",
			updateEntries: []Entry{entry1},
			expectStatus:  []Status{{Code: codes.NotFound, Message: `entry "E1" not found`}},
		},
		{
			desc:          "less than a batch",
			withEntries:   []Entry{entry1},
			updateEntries: []Entry{entry1},
			expectEntries: []Entry{entry1},
			expectStatus:  []Status{ok},
		},
		{
			desc:          "exactly a batch",
			withEntries:   []Entry{entry1Old, entry2Old},
			updateEntries: []Entry{entry1, entry2},
			expectEntries: []Entry{entry1, entry2},
			expectStatus:  []Status{ok, ok},
		},
		{
			desc:          "more than a batch",
			withEntries:   []Entry{entry1Old, entry2Old, entry3Old},
			updateEntries: []Entry{entry1, entry2, entry3},
			expectEntries: []Entry{entry1, entry2, entry3},
			expectStatus:  []Status{ok, ok, ok},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			server.setEntries(t, tc.withEntries...)
			server.batchUpdateEntriesErr = tc.expectErr
			actualStatus, err := client.UpdateEntries(ctx, tc.updateEntries)
			if tc.expectErr != nil {
				assertErrorIs(t, err, tc.expectErr)
				assert.Empty(t, actualStatus)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectStatus, actualStatus)
			assert.ElementsMatch(t, tc.expectEntries, server.getEntries(t))
		})
	}
}

func TestDeleteEntries(t *testing.T) {
	server, client := startEntryAPIServer(t)

	ok := Status{Code: codes.OK}

	for _, tc := range []struct {
		desc          string
		withEntries   []Entry
		deleteEntries []string
		expectEntries []Entry
		expectStatus  []Status
		expectErr     error
	}{
		{
			desc:          "empty",
			expectEntries: nil,
			expectStatus:  []Status{},
		},
		{
			desc:          "RPC error",
			deleteEntries: []string{entry1ID},
			expectErr:     status.Error(codes.Internal, "oh no"),
		},
		{
			desc:          "not found",
			deleteEntries: []string{entry1ID},
			expectStatus:  []Status{{Code: codes.NotFound, Message: `entry "E1" not found`}},
		},
		{
			desc:          "less than a batch",
			withEntries:   []Entry{entry1},
			deleteEntries: []string{entry1ID},
			expectStatus:  []Status{ok},
		},
		{
			desc:          "exactly a batch",
			withEntries:   []Entry{entry1, entry2},
			deleteEntries: []string{entry1ID, entry2ID},
			expectStatus:  []Status{ok, ok},
		},
		{
			desc:          "more than a batch",
			withEntries:   []Entry{entry1, entry2, entry3},
			deleteEntries: []string{entry1ID, entry2ID, entry3ID},
			expectStatus:  []Status{ok, ok, ok},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			server.setEntries(t, tc.withEntries...)
			server.batchDeleteEntriesErr = tc.expectErr
			actualStatus, err := client.DeleteEntries(ctx, tc.deleteEntries)
			if tc.expectErr != nil {
				assertErrorIs(t, err, tc.expectErr)
				assert.Empty(t, actualStatus)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectStatus, actualStatus)
			assert.ElementsMatch(t, tc.expectEntries, server.getEntries(t))
		})
	}
}

func startEntryAPIServer(t *testing.T) (*entryServer, EntryClient) {
	api := &entryServer{}
	conn := startServer(t, func(s *grpc.Server) {
		entryv1.RegisterEntryServer(s, api)
	})
	return api, NewEntryClient(conn)
}

type entryServer struct {
	entryv1.UnimplementedEntryServer

	mtx     sync.RWMutex
	entries []*apitypes.Entry

	listEntriesErr        error
	batchCreateEntriesErr error
	batchUpdateEntriesErr error
	batchDeleteEntriesErr error
}

func (s *entryServer) ListEntries(ctx context.Context, req *entryv1.ListEntriesRequest) (*entryv1.ListEntriesResponse, error) {
	resp := new(entryv1.ListEntriesResponse)

	s.mtx.RLock()
	defer s.mtx.RUnlock()

	start, end, more := listBounds(req.PageToken, int(req.PageSize), len(s.entries), func(i int) string { return s.entries[i].Id })
	for _, entry := range s.entries[start:end] {
		resp.Entries = append(resp.Entries, entry)
		if more {
			resp.NextPageToken = entry.Id
		}
	}

	return resp, s.listEntriesErr
}

func (s *entryServer) BatchCreateEntry(ctx context.Context, req *entryv1.BatchCreateEntryRequest) (*entryv1.BatchCreateEntryResponse, error) {
	resp := new(entryv1.BatchCreateEntryResponse)

	for _, entry := range req.Entries {
		st := status.Convert(s.createEntry(entry))
		result := &entryv1.BatchCreateEntryResponse_Result{
			Status: &apitypes.Status{
				Code:    int32(st.Code()),
				Message: st.Message(),
			},
		}
		if st.Code() == codes.OK {
			result.Entry = entry
		}
		resp.Results = append(resp.Results, result)
	}

	return resp, s.batchCreateEntriesErr
}

func (s *entryServer) BatchUpdateEntry(ctx context.Context, req *entryv1.BatchUpdateEntryRequest) (*entryv1.BatchUpdateEntryResponse, error) {
	resp := new(entryv1.BatchUpdateEntryResponse)

	for _, entry := range req.Entries {
		st := status.Convert(s.updateEntry(entry))
		result := &entryv1.BatchUpdateEntryResponse_Result{
			Status: &apitypes.Status{
				Code:    int32(st.Code()),
				Message: st.Message(),
			},
		}
		if st.Code() == codes.OK {
			result.Entry = entry
		}
		resp.Results = append(resp.Results, result)
	}
	return resp, s.batchUpdateEntriesErr
}

func (s *entryServer) BatchDeleteEntry(ctx context.Context, req *entryv1.BatchDeleteEntryRequest) (*entryv1.BatchDeleteEntryResponse, error) {
	resp := new(entryv1.BatchDeleteEntryResponse)

	for _, id := range req.Ids {
		st := status.Convert(s.deleteEntry(id))
		result := &entryv1.BatchDeleteEntryResponse_Result{
			Status: &apitypes.Status{
				Code:    int32(st.Code()),
				Message: st.Message(),
			},
			Id: id,
		}
		resp.Results = append(resp.Results, result)
	}
	return resp, s.batchDeleteEntriesErr
}

func (s *entryServer) clearEntries() {
	s.mtx.Lock()
	s.entries = nil
	s.mtx.Unlock()
}

func (s *entryServer) getEntries(t *testing.T) []Entry {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	entries, err := entriesFromAPI(s.entries)
	require.NoError(t, err)
	return entries
}

func (s *entryServer) setEntries(t *testing.T, entries ...Entry) {
	s.clearEntries()
	for _, entry := range entries {
		err := s.createEntry(entryToAPI(entry))
		require.NoError(t, err, "test setup failure creating entry")
	}
}

func (s *entryServer) createEntry(entry *apitypes.Entry) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	n := sort.Search(len(s.entries), func(i int) bool {
		return s.entries[i].Id >= entry.Id
	})
	if n < len(s.entries) && s.entries[n].Id == entry.Id {
		return status.Errorf(codes.AlreadyExists, "entry %q already exists", entry.Id)
	}
	s.entries = append(s.entries[:n], append([]*apitypes.Entry{entry}, s.entries[n:]...)...)
	return nil
}

func (s *entryServer) updateEntry(entry *apitypes.Entry) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	n := sort.Search(len(s.entries), func(i int) bool {
		return s.entries[i].Id >= entry.Id
	})
	if !(n < len(s.entries) && s.entries[n].Id == entry.Id) {
		return status.Errorf(codes.NotFound, "entry %q not found", entry.Id)
	}
	s.entries[n] = entry
	return nil
}

func (s *entryServer) deleteEntry(td string) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	n := sort.Search(len(s.entries), func(i int) bool {
		return s.entries[i].Id >= td
	})
	if !(n < len(s.entries) && s.entries[n].Id == td) {
		return status.Errorf(codes.NotFound, "entry %q not found", td)
	}
	s.entries = s.entries[:n+copy(s.entries[n:], s.entries[n+1:])]
	return nil
}
