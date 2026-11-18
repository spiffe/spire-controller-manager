package main

import (
	"net"
	"net/http"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/stretchr/testify/require"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func TestStaticRunServesHealthProbesAndStopsOnSignal(t *testing.T) {
	healthAddr := freeTCPAddr(t)
	staticManifestPath := t.TempDir()

	mainConfig := Config{
		ctrlConfig: spirev1alpha1.ControllerManagerConfig{
			TrustDomain:           "example.org",
			GCInterval:            time.Hour,
			SPIREServerSocketPath: filepath.Join(t.TempDir(), "api.sock"),
			StaticManifestPath:    &staticManifestPath,
		},
		options: ctrl.Options{
			Scheme:                 scheme,
			HealthProbeBindAddress: healthAddr,
			Metrics:                metricsserver.Options{BindAddress: "0"},
		},
		reconcile: spirev1alpha1.ReconcileConfig{
			ClusterStaticEntries:         true,
			ClusterFederatedTrustDomains: true,
		},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- staticRun(mainConfig)
	}()

	// The static reconcilers run until shutdown, so the health probes must be
	// served while they are running.
	require.Eventually(t, func() bool {
		resp, err := http.Get("http://" + healthAddr + "/healthz")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 10*time.Second, 50*time.Millisecond, "health probe endpoint was not served")

	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))

	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		require.FailNow(t, "staticRun did not return after SIGTERM")
	}
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}
