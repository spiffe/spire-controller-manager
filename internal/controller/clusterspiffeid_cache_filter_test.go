package controller

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// cacheItemCount starts a cache against restCfg with an optional label selector,
// waits for it to sync, and returns the number of cached items.
func cacheItemCount(ctx context.Context, t *testing.T, restCfg *rest.Config, scheme *k8sruntime.Scheme, selector labels.Selector) int {
	t.Helper()

	cacheOpts := cache.Options{Scheme: scheme}
	if selector != nil {
		cacheOpts.ByObject = map[client.Object]cache.ByObject{
			&spirev1alpha1.ClusterSPIFFEID{}: {Label: selector},
		}
	}

	c, err := cache.New(restCfg, cacheOpts)
	require.NoError(t, err)

	cacheCtx, cancel := context.WithCancel(ctx)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		_ = c.Start(cacheCtx)
	}()

	synced := c.WaitForCacheSync(cacheCtx)
	require.True(t, synced, "cache did not sync")

	var list spirev1alpha1.ClusterSPIFFEIDList
	require.NoError(t, c.List(cacheCtx, &list))

	cancel()
	<-stopped
	return len(list.Items)
}

func newFilterTestScheme(t *testing.T) *k8sruntime.Scheme {
	t.Helper()
	scheme := k8sruntime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(spirev1alpha1.AddToScheme(scheme))
	return scheme
}

func startFilterTestEnv(t *testing.T) *rest.Config {
	t.Helper()

	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.28.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}

	restCfg, err := env.Start()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, env.Stop()) })
	return restCfg
}

// createClusterSPIFFEIDsRange creates objects in [from, to). The first object
// (index 0) always carries the child-server label; all others have no labels.
// This mirrors production: 1 child-server entry among many workload entries.
func createClusterSPIFFEIDsRange(ctx context.Context, t *testing.T, c client.Client, from, to int) {
	t.Helper()
	for i := from; i < to; i++ {
		obj := &spirev1alpha1.ClusterSPIFFEID{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("test-spiffeid-%05d", i),
			},
			Spec: spirev1alpha1.ClusterSPIFFEIDSpec{
				SPIFFEIDTemplate: fmt.Sprintf("spiffe://test/workload-%d", i),
			},
		}
		if i == 0 {
			obj.Labels = map[string]string{"spire.spiffe.io/child-server": "true"}
		}
		require.NoError(t, c.Create(ctx, obj))
	}
}

func deleteClusterSPIFFEIDsRange(ctx context.Context, t *testing.T, c client.Client, from, to int) {
	t.Helper()
	for i := from; i < to; i++ {
		obj := &spirev1alpha1.ClusterSPIFFEID{}
		key := types.NamespacedName{Name: fmt.Sprintf("test-spiffeid-%05d", i)}
		if err := c.Get(ctx, key, obj); err == nil {
			_ = c.Delete(ctx, obj)
		}
	}
}

// createLabeledClusterSPIFFEID creates a single ClusterSPIFFEID with the given
// name and labels (nil/empty for no labels), and registers cleanup.
func createLabeledClusterSPIFFEID(ctx context.Context, t *testing.T, c client.Client, name string, objLabels map[string]string) {
	t.Helper()
	obj := &spirev1alpha1.ClusterSPIFFEID{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: objLabels,
		},
		Spec: spirev1alpha1.ClusterSPIFFEIDSpec{
			SPIFFEIDTemplate: fmt.Sprintf("spiffe://test/%s", name),
		},
	}
	require.NoError(t, c.Create(ctx, obj))
	t.Cleanup(func() {
		_ = c.Delete(ctx, obj)
	})
}

// TestClusterSPIFFEIDCacheFilter_Correctness verifies that the label selector
// restricts the cache to only matching objects.
func TestClusterSPIFFEIDCacheFilter_Correctness(t *testing.T) {
	const total = 200
	ctx := context.Background()
	restCfg := startFilterTestEnv(t)
	scheme := newFilterTestScheme(t)

	directClient, err := client.New(restCfg, client.Options{Scheme: scheme})
	require.NoError(t, err)

	createClusterSPIFFEIDsRange(ctx, t, directClient, 0, total)
	t.Cleanup(func() { deleteClusterSPIFFEIDsRange(ctx, t, directClient, 0, total) })

	t.Run("unfiltered cache returns all objects", func(t *testing.T) {
		count := cacheItemCount(ctx, t, restCfg, scheme, nil)
		require.Equal(t, total, count)
	})

	t.Run("filtered cache returns only the single labeled object", func(t *testing.T) {
		sel := labels.SelectorFromSet(labels.Set{"spire.spiffe.io/child-server": "true"})
		count := cacheItemCount(ctx, t, restCfg, scheme, sel)
		require.Equal(t, 1, count)
	})
}

// TestClusterSPIFFEIDCacheFilter_CombinedSelector demonstrates the effective
// behavior when clusterSPIFFEIDLabelSelector and filterByClassName are both
// configured: the resulting cache label selector is an AND of every key
// involved (labels.SelectorFromSet builds one equality requirement per key),
// so an object must carry every label to be cached. It is not sufficient
// for an object to satisfy only one of the two label sources.
func TestClusterSPIFFEIDCacheFilter_CombinedSelector(t *testing.T) {
	ctx := context.Background()
	restCfg := startFilterTestEnv(t)
	scheme := newFilterTestScheme(t)

	directClient, err := client.New(restCfg, client.Options{Scheme: scheme})
	require.NoError(t, err)

	// Mirrors clusterSPIFFEIDLabelSelector: {"spire.spiffe.io/child-server": "true"}
	// combined with filterByClassName deriving {"spire.spiffe.io/class-name": "spire-mgmt-external-server"}.
	combinedSelector := labels.SelectorFromSet(labels.Set{
		"spire.spiffe.io/child-server": "true",
		"spire.spiffe.io/class-name":   "spire-mgmt-external-server",
	})

	createLabeledClusterSPIFFEID(ctx, t, directClient, "both-labels", map[string]string{
		"spire.spiffe.io/child-server": "true",
		"spire.spiffe.io/class-name":   "spire-mgmt-external-server",
	})
	createLabeledClusterSPIFFEID(ctx, t, directClient, "only-child-server-label", map[string]string{
		"spire.spiffe.io/child-server": "true",
	})
	createLabeledClusterSPIFFEID(ctx, t, directClient, "only-class-name-label", map[string]string{
		"spire.spiffe.io/class-name": "spire-mgmt-external-server",
	})
	createLabeledClusterSPIFFEID(ctx, t, directClient, "no-labels", nil)

	t.Run("only the object carrying both labels is cached", func(t *testing.T) {
		count := cacheItemCount(ctx, t, restCfg, scheme, combinedSelector)
		require.Equal(t, 1, count)
	})

	t.Run("an object missing either label is excluded", func(t *testing.T) {
		cacheOpts := cache.Options{Scheme: scheme, ByObject: map[client.Object]cache.ByObject{
			&spirev1alpha1.ClusterSPIFFEID{}: {Label: combinedSelector},
		}}
		c, err := cache.New(restCfg, cacheOpts)
		require.NoError(t, err)

		cacheCtx, cancel := context.WithCancel(ctx)
		stopped := make(chan struct{})
		go func() {
			defer close(stopped)
			_ = c.Start(cacheCtx)
		}()
		require.True(t, c.WaitForCacheSync(cacheCtx), "cache did not sync")
		defer func() {
			cancel()
			<-stopped
		}()

		for _, name := range []string{"only-child-server-label", "only-class-name-label", "no-labels"} {
			var obj spirev1alpha1.ClusterSPIFFEID
			err := c.Get(cacheCtx, types.NamespacedName{Name: name}, &obj)
			require.Error(t, err, "expected %q to be excluded from the cache", name)
		}

		var obj spirev1alpha1.ClusterSPIFFEID
		require.NoError(t, c.Get(cacheCtx, types.NamespacedName{Name: "both-labels"}, &obj))
	})
}
