package spireentry

import (
	"testing"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/spiffe/spire-controller-manager/pkg/spireapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestPodEntryCacheKey(t *testing.T) {
	t.Run("returns string of UID", func(t *testing.T) {
		uid := types.UID("abc-123")
		assert.Equal(t, "abc-123", podEntryCacheKey(uid))
	})

	t.Run("different UIDs produce different keys", func(t *testing.T) {
		assert.NotEqual(t,
			podEntryCacheKey(types.UID("uid-1")),
			podEntryCacheKey(types.UID("uid-2")),
		)
	})
}

func TestCachedEntryIsValid(t *testing.T) {
	entry := &cachedEntry{
		podRV:       "100",
		nodeRV:      "200",
		specHash:    "spec-abc",
		endpointsRV: "300,301",
	}

	t.Run("all fields match", func(t *testing.T) {
		assert.True(t, entry.isValid("100", "200", "spec-abc", "300,301"))
	})

	t.Run("pod RV changed", func(t *testing.T) {
		assert.False(t, entry.isValid("101", "200", "spec-abc", "300,301"))
	})

	t.Run("node RV changed", func(t *testing.T) {
		assert.False(t, entry.isValid("100", "201", "spec-abc", "300,301"))
	})

	t.Run("spec hash changed", func(t *testing.T) {
		assert.False(t, entry.isValid("100", "200", "spec-def", "300,301"))
	})

	t.Run("endpoints RV changed", func(t *testing.T) {
		assert.False(t, entry.isValid("100", "200", "spec-abc", "302,301"))
	})

	t.Run("empty endpoints RV matches empty", func(t *testing.T) {
		e := &cachedEntry{podRV: "1", nodeRV: "2", specHash: "s", endpointsRV: ""}
		assert.True(t, e.isValid("1", "2", "s", ""))
	})
}

func TestComputeEndpointsRV(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		assert.Equal(t, "", computeEndpointsRV(nil))
		assert.Equal(t, "", computeEndpointsRV([]corev1.Endpoints{}))
	})

	t.Run("single endpoint", func(t *testing.T) {
		items := []corev1.Endpoints{
			{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "100"}},
		}
		assert.Equal(t, "100", computeEndpointsRV(items))
	})

	t.Run("multiple endpoints are comma separated", func(t *testing.T) {
		items := []corev1.Endpoints{
			{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "100"}},
			{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "200"}},
			{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "300"}},
		}
		assert.Equal(t, "100,200,300", computeEndpointsRV(items))
	})

	t.Run("RV change produces different result", func(t *testing.T) {
		before := []corev1.Endpoints{
			{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "100"}},
		}
		after := []corev1.Endpoints{
			{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "101"}},
		}
		assert.NotEqual(t, computeEndpointsRV(before), computeEndpointsRV(after))
	})
}

func TestComputeObjectHash(t *testing.T) {
	t.Run("same object produces same hash", func(t *testing.T) {
		h1, err := computeObjectHash(map[string]string{"a": "b"})
		require.NoError(t, err)
		h2, err := computeObjectHash(map[string]string{"a": "b"})
		require.NoError(t, err)
		assert.Equal(t, h1, h2)
	})

	t.Run("different objects produce different hashes", func(t *testing.T) {
		h1, err := computeObjectHash(map[string]string{"a": "b"})
		require.NoError(t, err)
		h2, err := computeObjectHash(map[string]string{"a": "c"})
		require.NoError(t, err)
		assert.NotEqual(t, h1, h2)
	})
}

func TestLRUCacheIntegration(t *testing.T) {
	cache, err := lru.New[string, *cachedEntry](10)
	require.NoError(t, err)

	dummyEntry := &spireapi.Entry{ID: "entry-1"}

	t.Run("cache miss on empty cache", func(t *testing.T) {
		_, ok := cache.Get(podEntryCacheKey("pod-1"))
		assert.False(t, ok)
	})

	t.Run("cache hit after add", func(t *testing.T) {
		key := podEntryCacheKey("pod-1")
		cache.Add(key, &cachedEntry{
			podRV:       "100",
			nodeRV:      "200",
			specHash:    "spec-1",
			endpointsRV: "",
			entry:       dummyEntry,
		})

		cached, ok := cache.Get(key)
		require.True(t, ok)
		assert.True(t, cached.isValid("100", "200", "spec-1", ""))
		assert.Equal(t, dummyEntry, cached.entry)
	})

	t.Run("cache invalidated by pod RV change", func(t *testing.T) {
		key := podEntryCacheKey("pod-1")
		cached, ok := cache.Get(key)
		require.True(t, ok)
		assert.False(t, cached.isValid("101", "200", "spec-1", ""))
	})

	t.Run("overwrite same key updates cached entry", func(t *testing.T) {
		key := podEntryCacheKey("pod-1")
		newEntry := &spireapi.Entry{ID: "entry-1-updated"}
		cache.Add(key, &cachedEntry{
			podRV:       "101",
			nodeRV:      "200",
			specHash:    "spec-1",
			endpointsRV: "",
			entry:       newEntry,
		})

		cached, ok := cache.Get(key)
		require.True(t, ok)
		assert.True(t, cached.isValid("101", "200", "spec-1", ""))
		assert.Equal(t, newEntry, cached.entry)
	})

	t.Run("different pods have independent cache entries", func(t *testing.T) {
		entry2 := &spireapi.Entry{ID: "entry-2"}
		cache.Add(podEntryCacheKey("pod-2"), &cachedEntry{
			podRV:       "500",
			nodeRV:      "600",
			specHash:    "spec-2",
			endpointsRV: "700",
			entry:       entry2,
		})

		cached1, ok := cache.Get(podEntryCacheKey("pod-1"))
		require.True(t, ok)
		assert.Equal(t, "101", cached1.podRV)

		cached2, ok := cache.Get(podEntryCacheKey("pod-2"))
		require.True(t, ok)
		assert.Equal(t, "500", cached2.podRV)
		assert.Equal(t, entry2, cached2.entry)
	})

	t.Run("LRU eviction when cache is full", func(t *testing.T) {
		smallCache, err := lru.New[string, *cachedEntry](2)
		require.NoError(t, err)

		smallCache.Add("a", &cachedEntry{podRV: "1", entry: &spireapi.Entry{ID: "a"}})
		smallCache.Add("b", &cachedEntry{podRV: "2", entry: &spireapi.Entry{ID: "b"}})
		smallCache.Add("c", &cachedEntry{podRV: "3", entry: &spireapi.Entry{ID: "c"}})

		_, ok := smallCache.Get("a")
		assert.False(t, ok, "oldest entry should be evicted")

		_, ok = smallCache.Get("b")
		assert.True(t, ok)

		_, ok = smallCache.Get("c")
		assert.True(t, ok)
	})
}
