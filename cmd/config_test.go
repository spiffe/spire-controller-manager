package main

import (
	"errors"
	"testing"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/stretchr/testify/require"
)

func TestParseClusterDomainCNAME(t *testing.T) {
	for _, test := range []struct {
		name           string
		cname          string
		expectedDomain string
		expectedErr    string
	}{
		{
			name:           "Valid CNAME with trailing dot",
			cname:          k8sDefaultService + ".cluster.local.",
			expectedDomain: "cluster.local",
		},
		{
			name:           "Valid CNAME without trailing dot",
			cname:          k8sDefaultService + ".cluster2.local",
			expectedDomain: "cluster2.local",
		},
		{
			name:        "Invalid prefix",
			cname:       "test.cluster.local",
			expectedErr: "CNAME did not have expected prefix",
		},
		{
			name:        "No domain with trailing dot",
			cname:       k8sDefaultService + ".",
			expectedErr: "CNAME did not have a cluster domain",
		},
		{
			name:        "No domain without trailing dot",
			cname:       k8sDefaultService,
			expectedErr: "CNAME did not have expected prefix",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			domain, err := parseClusterDomainCNAME(test.cname)
			if test.expectedErr != "" {
				require.EqualError(t, errors.New(test.expectedErr), err.Error())
				return
			}

			require.Equal(t, domain, test.expectedDomain)
			require.NoError(t, err)
		})
	}
}

func TestClusterSPIFFEIDCacheLabels(t *testing.T) {
	for _, test := range []struct {
		name           string
		ctrlConfig     spirev1alpha1.ControllerManagerConfig
		expectedLabels map[string]string
	}{
		{
			name:           "neither set",
			ctrlConfig:     spirev1alpha1.ControllerManagerConfig{},
			expectedLabels: nil,
		},
		{
			name: "only ClusterSPIFFEIDLabelSelector set",
			ctrlConfig: spirev1alpha1.ControllerManagerConfig{
				ControllerManagerConfigurationSpec: spirev1alpha1.ControllerManagerConfigurationSpec{
					ClusterSPIFFEIDLabelSelector: map[string]string{
						"spire.spiffe.io/child-server": "true",
					},
				},
			},
			expectedLabels: map[string]string{
				"spire.spiffe.io/child-server": "true",
			},
		},
		{
			name: "only FilterByClassName set",
			ctrlConfig: spirev1alpha1.ControllerManagerConfig{
				ControllerManagerConfigurationSpec: spirev1alpha1.ControllerManagerConfigurationSpec{
					ClassName:         "spire-mgmt-external-server",
					FilterByClassName: true,
				},
			},
			expectedLabels: map[string]string{
				"spire.spiffe.io/class-name": "spire-mgmt-external-server",
			},
		},
		{
			name: "both set, merged",
			ctrlConfig: spirev1alpha1.ControllerManagerConfig{
				ControllerManagerConfigurationSpec: spirev1alpha1.ControllerManagerConfigurationSpec{
					ClassName:         "spire-mgmt-external-server",
					FilterByClassName: true,
					ClusterSPIFFEIDLabelSelector: map[string]string{
						"spire.spiffe.io/child-server": "true",
					},
				},
			},
			expectedLabels: map[string]string{
				"spire.spiffe.io/child-server": "true",
				"spire.spiffe.io/class-name":   "spire-mgmt-external-server",
			},
		},
		{
			name: "both set, FilterByClassName overwrites conflicting selector value",
			ctrlConfig: spirev1alpha1.ControllerManagerConfig{
				ControllerManagerConfigurationSpec: spirev1alpha1.ControllerManagerConfigurationSpec{
					ClassName:         "spire-mgmt-external-server",
					FilterByClassName: true,
					ClusterSPIFFEIDLabelSelector: map[string]string{
						"spire.spiffe.io/class-name": "some-other-value",
					},
				},
			},
			expectedLabels: map[string]string{
				"spire.spiffe.io/class-name": "spire-mgmt-external-server",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expectedLabels, clusterSPIFFEIDCacheLabels(test.ctrlConfig))
		})
	}
}
