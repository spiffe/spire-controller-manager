package v1alpha1_test

import (
	"os"
	"path/filepath"
	"testing"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	cseFederatedOKFileContent = `
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterFederatedTrustDomain
spec:
  bundleEndpointProfile:
    type: https_web
  bundleEndpointURL: https://example.org
  trustDomain: example.org
`
	cseOKFileContent = `
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterStaticEntry
spec:
  parentID: spiffe://example.org/server
  spiffeID: spiffe://example.org/test
  selectors:
  - test:123
`
	cseFederatedNotOKFileContent = `
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterFederatedTrustDomain
spec:
  bundleEndpointProfile:
  - type: https_web
  bundleEndpointURL: https://example.org
  trustDomain: example.org
`
	cseNotOKFileContent = `
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterStaticEntry
spec:
  parentID: spiffe://example.org/server
  selectors:
    test: 123
    other: thingy
`
)

func TestStaticLoadOkConfig(t *testing.T) {
	scheme := runtime.NewScheme()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "file.yaml")
	require.NoError(t, os.WriteFile(path, []byte(cseOKFileContent), 0600))

	cse, err := spirev1alpha1.LoadClusterStaticEntryFile(path, scheme, false)
	require.NoError(t, err)

	expectConfig := &spirev1alpha1.ClusterStaticEntry{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterStaticEntry",
			APIVersion: "spire.spiffe.io/v1alpha1",
		},
		Spec: spirev1alpha1.ClusterStaticEntrySpec{
			ParentID:  "spiffe://example.org/server",
			SPIFFEID:  "spiffe://example.org/test",
			Selectors: []string{"test:123"},
		},
	}
	require.Equal(t, expectConfig, cse)
}

func TestStaticLoadStaticOkConfig(t *testing.T) {
	scheme := runtime.NewScheme()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "file.yaml")
	require.NoError(t, os.WriteFile(path, []byte(cseFederatedOKFileContent), 0600))

	cse, err := spirev1alpha1.LoadClusterStaticEntryFile(path, scheme, false)
	require.NoError(t, err)
	require.Equal(t, cse.APIVersion, "spire.spiffe.io/v1alpha1")
	require.Equal(t, cse.Kind, "ClusterFederatedTrustDomain")
}

func TestStaticLoadNotOkConfig(t *testing.T) {
	scheme := runtime.NewScheme()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "file.yaml")
	require.NoError(t, os.WriteFile(path, []byte(cseNotOKFileContent), 0600))

	_, err := spirev1alpha1.LoadClusterStaticEntryFile(path, scheme, false)
	require.Error(t, err)
}

func TestStaticLoadStaticNotOkConfig(t *testing.T) {
	scheme := runtime.NewScheme()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "file.yaml")
	require.NoError(t, os.WriteFile(path, []byte(cseFederatedNotOKFileContent), 0600))

	cse, err := spirev1alpha1.LoadClusterStaticEntryFile(path, scheme, false)
	require.NoError(t, err)
	require.Equal(t, cse.APIVersion, "spire.spiffe.io/v1alpha1")
	require.Equal(t, cse.Kind, "ClusterFederatedTrustDomain")
}
