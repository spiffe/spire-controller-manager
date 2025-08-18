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
	cftdOKFileContent = `
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterFederatedTrustDomain
spec:
  bundleEndpointProfile:
    type: https_web
  bundleEndpointURL: https://example.org
  trustDomain: example.org
`
	cftdStaticOKFileContent = `
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterStaticEntry
spec:
  parentID: spiffe://example.org/server
  spiffeID: spiffe://example.org/test
  selectors:
  - test:123
`
	cftdNotOKFileContent = `
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterFederatedTrustDomain
spec:
  bundleEndpointProfile:
  - type: https_web
  bundleEndpointURL: https://example.org
  trustDomain: example.org
`
	cftdStaticNotOKFileContent = `
apiVersion: spire.spiffe.io/v1alpha1
kind: ClusterStaticEntry
spec:
  parentID: spiffe://example.org/server
  selectors:
    test: 123
    other: thingy
`
)

func TestFederatedLoadOkConfig(t *testing.T) {
	scheme := runtime.NewScheme()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "file.yaml")
	require.NoError(t, os.WriteFile(path, []byte(cftdOKFileContent), 0600))

	cftd, err := spirev1alpha1.LoadClusterFederatedTrustDomainFile(path, scheme, false)
	require.NoError(t, err)

	expectConfig := &spirev1alpha1.ClusterFederatedTrustDomain{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterFederatedTrustDomain",
			APIVersion: "spire.spiffe.io/v1alpha1",
		},
		Spec: spirev1alpha1.ClusterFederatedTrustDomainSpec{
			BundleEndpointProfile: spirev1alpha1.BundleEndpointProfile{
				Type: "https_web",
			},
			BundleEndpointURL: "https://example.org",
			TrustDomain:       "example.org",
		},
	}
	require.Equal(t, expectConfig, cftd)
}

func TestFederatedLoadStaticOkConfig(t *testing.T) {
	scheme := runtime.NewScheme()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "file.yaml")
	require.NoError(t, os.WriteFile(path, []byte(cftdStaticOKFileContent), 0600))

	cftd, err := spirev1alpha1.LoadClusterFederatedTrustDomainFile(path, scheme, false)
	require.NoError(t, err)
	require.Equal(t, cftd.APIVersion, "spire.spiffe.io/v1alpha1")
	require.Equal(t, cftd.Kind, "ClusterStaticEntry")
}

func TestFederatedLoadNotOkConfig(t *testing.T) {
	scheme := runtime.NewScheme()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "file.yaml")
	require.NoError(t, os.WriteFile(path, []byte(cftdNotOKFileContent), 0600))

	cftd, err := spirev1alpha1.LoadClusterFederatedTrustDomainFile(path, scheme, false)
	require.Error(t, err)
	require.Equal(t, cftd, nil)
}

func TestFederatedLoadStaticNotOkConfig(t *testing.T) {
	scheme := runtime.NewScheme()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "file.yaml")
	require.NoError(t, os.WriteFile(path, []byte(cftdStaticNotOKFileContent), 0600))

	cftd, err := spirev1alpha1.LoadClusterFederatedTrustDomainFile(path, scheme, false)
	require.NoError(t, err)
	require.Equal(t, cftd.APIVersion, "spire.spiffe.io/v1alpha1")
	require.Equal(t, cftd.Kind, "ClusterStaticEntry")
}
