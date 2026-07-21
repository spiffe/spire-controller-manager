//go:build benchmark

package controller

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// liveHeapAlloc starts a cache against restCfg, waits for it to sync, then
// measures live HeapAlloc while the cache is running (after two GC passes).
// Returns heap bytes and the number of items in the cache.
func liveHeapAlloc(ctx context.Context, t *testing.T, restCfg *rest.Config, scheme *k8sruntime.Scheme, selector labels.Selector) (heapAlloc uint64, itemCount int) {
	t.Helper()

	cacheOpts := cache.Options{Scheme: scheme}
	if selector != nil {
		cacheOpts.ByObject = map[client.Object]cache.ByObject{
			&spirev1alpha1.ClusterSPIFFEID{}: {Label: selector},
		}
	}

	c, err := cache.New(restCfg, cacheOpts)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	cacheCtx, cancel := context.WithCancel(ctx)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		_ = c.Start(cacheCtx)
	}()

	synced := c.WaitForCacheSync(cacheCtx)
	if !synced {
		t.Fatal("cache did not sync")
	}

	var list spirev1alpha1.ClusterSPIFFEIDList
	if err := c.List(cacheCtx, &list); err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	itemCount = len(list.Items)

	runtime.GC()
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	heapAlloc = stats.HeapAlloc

	cancel()
	<-stopped
	return heapAlloc, itemCount
}

// benchStep holds the result for one step of the scaling benchmark.
type benchStep struct {
	total          int
	unfilteredHeap uint64
	filteredHeap   uint64
}

// TestClusterSPIFFEIDCacheFilter_ScalingBenchmark measures cache heap usage at
// evenly-spaced object counts. In each step the total ClusterSPIFFEID count
// increases, but the filtered set stays fixed at exactly 1 labeled entry.
// This mirrors scenarios where a controller manager instance only needs a small
// subset of all ClusterSPIFFEIDs in the cluster.
//
// Run with:
//
//	go test ./internal/controller/... -tags benchmark -run TestClusterSPIFFEIDCacheFilter_ScalingBenchmark -v
//
// Expected observations:
//  1. Unfiltered heap grows proportionally with object count.
//  2. Filtered heap stays roughly constant regardless of total count.
func TestClusterSPIFFEIDCacheFilter_ScalingBenchmark(t *testing.T) {
	steps := []int{500, 1000, 2000, 4000}
	ctx := context.Background()
	restCfg := startFilterTestEnv(t)
	scheme := newFilterTestScheme(t)
	sel := labels.SelectorFromSet(labels.Set{"spire.spiffe.io/child-server": "true"})

	directClient, err := client.New(restCfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	results := make([]benchStep, 0, len(steps))
	prev := 0

	for _, total := range steps {
		createClusterSPIFFEIDsRange(ctx, t, directClient, prev, total)
		prev = total

		unfilteredHeap, unfilteredCount := liveHeapAlloc(ctx, t, restCfg, scheme, nil)
		runtime.GC()
		runtime.GC()
		filteredHeap, filteredCount := liveHeapAlloc(ctx, t, restCfg, scheme, sel)
		runtime.GC()
		runtime.GC()

		if unfilteredCount != total {
			t.Errorf("step %d: unfiltered count = %d, want %d", total, unfilteredCount, total)
		}
		if filteredCount != 1 {
			t.Errorf("step %d: filtered count = %d, want 1", total, filteredCount)
		}

		results = append(results, benchStep{
			total:          total,
			unfilteredHeap: unfilteredHeap,
			filteredHeap:   filteredHeap,
		})
	}

	t.Cleanup(func() { deleteClusterSPIFFEIDsRange(ctx, t, directClient, 0, steps[len(steps)-1]) })

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%-10s  %-18s  %-18s  %-10s\n",
		"Total", "Unfiltered (B)", "Filtered (B)", "Ratio"))
	sb.WriteString(strings.Repeat("-", 65) + "\n")
	for _, r := range results {
		ratio := float64(0)
		if r.filteredHeap > 0 {
			ratio = float64(r.unfilteredHeap) / float64(r.filteredHeap)
		}
		sb.WriteString(fmt.Sprintf("%-10d  %-18d  %-18d  %.2fx\n",
			r.total, r.unfilteredHeap, r.filteredHeap, ratio))
	}
	t.Log(sb.String())
}