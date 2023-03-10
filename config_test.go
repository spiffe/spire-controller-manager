package main

import (
	"errors"
	"testing"

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
