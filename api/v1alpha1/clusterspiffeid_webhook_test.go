package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDuplicateDNSNameTemplateGetsError(t *testing.T) {
	spec := &ClusterSPIFFEIDSpec{
		SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
		DNSNameTemplates: []string{
			"{{ .PodMeta.Name }}.{{ .PodMeta.Namespace }}.svc",
			"{{.PodSpec.ServiceAccountName }}.{{ .PodMeta.Namespace }}.svc",
			"{{.PodMeta.Name }}.{{ .PodMeta.Namespace }}.svc",
		},
	}

	_, err := ParseClusterSPIFFEIDSpec(spec)
	require.ErrorContains(t, err, "duplicate dnsNameTemplate: "+spec.DNSNameTemplates[2])
}
