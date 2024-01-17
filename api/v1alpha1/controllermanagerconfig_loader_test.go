package v1alpha1_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/component-base/config/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

const (
	fileContent = `
apiVersion: spire.spiffe.io/v1alpha1
kind: ControllerManagerConfig
metrics:
  bindAddress: 127.0.0.1:8082
health:
  healthProbeBindAddress: 127.0.0.1:8083
leaderElection:
  leaderElect: true
  resourceName: 98c9c988.spiffe.io
  resourceNamespace: spire-system
clusterName: cluster2
trustDomain: cluster2.demo
ignoreNamespaces:
  - kube-system
  - kube-public
  - spire-system
  - local-path-storage
`

	fileContentExpandEnv = `
apiVersion: spire.spiffe.io/v1alpha1
kind: ControllerManagerConfig
clusterName: cluster2
trustDomain: $TRUST_DOMAIN
`

	cacheNamespace = `
cacheNamespace: default
`
	cacheNamespaces = `
cacheNamespaces:
   default:
   nsWithLabel:
      labelSelectors:
         lName: l1
   nsWithField:
      fieldSelectors:
         fName: f1
   nsWithBoth:
      labelSelectors:
         lName: l1
      fieldSelectors:
         fName: f1
`
)

func TestLoadOptionsFromFileReplaceDefaultValues(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(spirev1alpha1.AddToScheme(scheme))

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(fileContent), 0600))

	options := ctrl.Options{Scheme: scheme}

	ctrlConfig := spirev1alpha1.ControllerManagerConfig{
		IgnoreNamespaces:                   []string{"kube-system", "kube-public", "spire-system", "foo"},
		GCInterval:                         time.Minute,
		ValidatingWebhookConfigurationName: "foo-webhook",
	}

	err := spirev1alpha1.LoadOptionsFromFile(path, scheme, &options, &ctrlConfig, false)
	require.NoError(t, err)

	ok := true
	expectConfig := spirev1alpha1.ControllerManagerConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ControllerManagerConfig",
			APIVersion: "spire.spiffe.io/v1alpha1",
		},
		ControllerManagerConfigurationSpec: spirev1alpha1.ControllerManagerConfigurationSpec{
			LeaderElection: &v1alpha1.LeaderElectionConfiguration{
				LeaderElect:       &ok,
				ResourceName:      "98c9c988.spiffe.io",
				ResourceNamespace: "spire-system",
			},
			Metrics: spirev1alpha1.ControllerMetrics{
				BindAddress: "127.0.0.1:8082",
			},
			Health: spirev1alpha1.ControllerHealth{
				HealthProbeBindAddress: "127.0.0.1:8083",
			},
		},
		ClusterName: "cluster2",
		TrustDomain: "cluster2.demo",
		IgnoreNamespaces: []string{
			"kube-system",
			"kube-public",
			"spire-system",
			"local-path-storage",
		},
		ValidatingWebhookConfigurationName: "foo-webhook",
		GCInterval:                         time.Minute,
	}
	require.Equal(t, expectConfig, ctrlConfig)

	require.Equal(t, "spire-system", options.LeaderElectionNamespace)
	require.True(t, true, options.LeaderElection)
	require.Equal(t, "98c9c988.spiffe.io", options.LeaderElectionID)
	require.Equal(t, "127.0.0.1:8082", options.Metrics.BindAddress)
}

func TestLoadOptionsFromFileInvalidPath(t *testing.T) {
	scheme := runtime.NewScheme()
	options := ctrl.Options{Scheme: scheme}

	ctrlConfig := spirev1alpha1.ControllerManagerConfig{
		IgnoreNamespaces:                   []string{"kube-system", "kube-public", "spire-system", "foo"},
		GCInterval:                         time.Minute,
		ValidatingWebhookConfigurationName: "foo-webhook",
	}

	err := spirev1alpha1.LoadOptionsFromFile("", scheme, &options, &ctrlConfig, false)
	require.EqualError(t, err, "could not read file at : open : no such file or directory")

	err = spirev1alpha1.LoadOptionsFromFile("foo.yaml", scheme, &options, &ctrlConfig, false)
	fmt.Printf("err :%v\n", err)
	require.EqualError(t, err, "could not read file at foo.yaml: open foo.yaml: no such file or directory")
}

func TestLoadOptionsFromFileExpandEnv(t *testing.T) {
	t.Setenv("TRUST_DOMAIN", "example.org")

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(fileContentExpandEnv), 0600))

	scheme := runtime.NewScheme()
	options := ctrl.Options{Scheme: scheme}

	ctrlConfig := spirev1alpha1.ControllerManagerConfig{}

	tests := []struct {
		expandEnv     bool
		expectedValue string
	}{
		{
			expandEnv:     true,
			expectedValue: "example.org",
		},
		{
			expandEnv:     false,
			expectedValue: "$TRUST_DOMAIN",
		},
	}

	for _, test := range tests {
		err := spirev1alpha1.LoadOptionsFromFile(path, scheme, &options, &ctrlConfig, test.expandEnv)
		require.NoError(t, err)
		require.Equal(t, test.expectedValue, ctrlConfig.TrustDomain)
	}
}

func TestLoadOptionsWithCacheNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(spirev1alpha1.AddToScheme(scheme))

	for _, tt := range []struct {
		name             string
		cacheNamespace   string
		expectErr        string
		expectNamespaces map[string]cache.Config
	}{
		{
			name:             "no namespace",
			expectNamespaces: nil,
		},
		{
			name:           "using namespaces",
			cacheNamespace: cacheNamespace,
			expectNamespaces: map[string]cache.Config{
				"default": {},
			},
		},
		{
			name:           "with cacheNamespaces",
			cacheNamespace: cacheNamespaces,
			expectNamespaces: map[string]cache.Config{
				"default": {},
				"nsWithLabel": {
					LabelSelector: labels.SelectorFromSet(labels.Set{
						"lName": "l1",
					}),
				},
				"nsWithField": {
					FieldSelector: fields.SelectorFromSet(fields.Set{
						"fName": "f1",
					}),
				},
				"nsWithBoth": {
					LabelSelector: labels.SelectorFromSet(labels.Set{
						"lName": "l1",
					}),
					FieldSelector: fields.SelectorFromSet(fields.Set{
						"fName": "f1",
					}),
				},
			},
		},
		{
			name:           "with cacheNamespace and cacheNamespaces",
			cacheNamespace: cacheNamespace + cacheNamespaces,
			expectErr:      "cacheNamespace or cacheNamespaces can be used, but not both",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			path := filepath.Join(tempDir, "config.yaml")

			config := fileContent + tt.cacheNamespace
			require.NoError(t, os.WriteFile(path, []byte(config), 0600))

			options := ctrl.Options{Scheme: scheme}

			ctrlConfig := spirev1alpha1.ControllerManagerConfig{
				IgnoreNamespaces:                   []string{"kube-system", "kube-public", "spire-system", "foo"},
				GCInterval:                         time.Minute,
				ValidatingWebhookConfigurationName: "foo-webhook",
			}

			err := spirev1alpha1.LoadOptionsFromFile(path, scheme, &options, &ctrlConfig, false)
			if tt.expectErr != "" {
				require.EqualError(t, err, tt.expectErr)
				return
			}

			require.NoError(t, err)

			require.Equal(t, tt.expectNamespaces, options.Cache.DefaultNamespaces)
		})
	}
}
