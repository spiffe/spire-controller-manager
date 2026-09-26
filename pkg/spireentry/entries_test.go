package spireentry

import (
	"fmt"
	"testing"
	"text/template"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterName   = "test"
	clusterDomain = "cluster.local"
	trustDomain   = "example.org"
)

func TestRenderPodEntry(t *testing.T) {
	spec := &spirev1alpha1.ClusterSPIFFEIDSpec{
		SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
		DNSNameTemplates: []string{
			"{{ .PodSpec.ServiceAccountName }}.{{ .PodMeta.Namespace }}.svc.{{ .ClusterDomain }}",
			"{{ .PodMeta.Name }}.{{ .PodMeta.Namespace }}.svc.{{ .ClusterDomain }}", // Duplicate
			"{{ .PodMeta.Name }}.{{ .TrustDomain }}.svc",
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			UID: "uid",
		},
		Spec: corev1.NodeSpec{},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "namespace",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "test",
		},
	}
	endpointsList := &corev1.EndpointsList{
		Items: []corev1.Endpoints{ //nolint: staticcheck // Refactor is going be done as part of a https://github.com/spiffe/spire-controller-manager/issues/554

			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "endpoint",
					Namespace: "namespace",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other-endpoint",
					Namespace: "namespace",
				},
			},
		},
	}

	parsedSpec, err := spirev1alpha1.ParseClusterSPIFFEIDSpec(spec)
	require.NoError(t, err)
	td, err := spiffeid.TrustDomainFromString(trustDomain)
	require.NoError(t, err)

	entry, err := renderPodEntry(parsedSpec, node, pod, endpointsList, td, clusterName, clusterDomain, nil)
	require.NoError(t, err)

	// SPIFFE ID rendered correctly
	spiffeID, err := spiffeid.FromPathf(td, "/ns/%s/sa/%s", pod.Namespace, pod.Spec.ServiceAccountName)
	require.NoError(t, err)
	require.Equal(t, entry.SPIFFEID.String(), spiffeID.String())

	// Parent ID rendered correctly
	parentID, err := spiffeid.FromPathf(td, "/spire/agent/k8s_psat/%s/%s", clusterName, node.UID)
	require.NoError(t, err)
	require.Equal(t, entry.ParentID.String(), parentID.String())

	// DNS names are unique
	dnsNamesSet := make(map[string]struct{})
	for _, dnsName := range entry.DNSNames {
		_, exists := dnsNamesSet[dnsName]
		require.False(t, exists)
		dnsNamesSet[dnsName] = struct{}{}
	}

	// DNS names list is as long as expected
	require.Equal(t, len(spec.DNSNameTemplates)-1+len(endpointsList.Items)*4, len(entry.DNSNames))

	// DNS names templates rendered correctly and are in order
	require.Equal(t, entry.DNSNames[0], pod.Spec.ServiceAccountName+"."+pod.Namespace+".svc."+clusterDomain)
	require.Equal(t, entry.DNSNames[1], pod.Name+"."+trustDomain+".svc")

	// Endpoint DNS Names auto populated
	for _, endpoint := range endpointsList.Items {
		require.Contains(t, entry.DNSNames, endpoint.Name)
		require.Contains(t, entry.DNSNames, endpoint.Name+"."+endpoint.Namespace)
		require.Contains(t, entry.DNSNames, endpoint.Name+"."+endpoint.Namespace+".svc")
		require.Contains(t, entry.DNSNames, endpoint.Name+"."+endpoint.Namespace+".svc."+clusterDomain)
	}
}

func TestJWTTTLInRenderPodEntry(t *testing.T) {
	spec := &spirev1alpha1.ClusterSPIFFEIDSpec{
		SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
		JWTTTL:           metav1.Duration{Duration: time.Duration(60)},
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			UID: "uid",
		},
		Spec: corev1.NodeSpec{},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "namespace",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "test",
		},
	}

	parsedSpec, err := spirev1alpha1.ParseClusterSPIFFEIDSpec(spec)
	require.NoError(t, err)
	td, err := spiffeid.TrustDomainFromString(trustDomain)
	require.NoError(t, err)

	entry, err := renderPodEntry(parsedSpec, node, pod, &corev1.EndpointsList{}, td, clusterName, clusterDomain, nil)
	require.NoError(t, err)

	require.Equal(t, entry.JWTSVIDTTL.Nanoseconds(), spec.JWTTTL.Nanoseconds())
}

func TestParentIDTemplateRenderPodEntry(t *testing.T) {
	spec := &spirev1alpha1.ClusterSPIFFEIDSpec{
		SPIFFEIDTemplate: "spiffe://{{ .TrustDomain }}/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}",
	}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			UID:  "uid",
			Name: "test.example.org",
		},
		Spec: corev1.NodeSpec{},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "namespace",
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: "test",
		},
	}

	defaultParentIDTemplate, err := template.New("testParentIDTemplate").Parse("spiffe://{{ .TrustDomain }}/spire/agent/x509pop/{{ .NodeMeta.Name }}")
	require.NoError(t, err)

	parsedSpec, err := spirev1alpha1.ParseClusterSPIFFEIDSpec(spec)
	require.NoError(t, err)
	td, err := spiffeid.TrustDomainFromString(trustDomain)
	require.NoError(t, err)

	entry, err := renderPodEntry(parsedSpec, node, pod, &corev1.EndpointsList{}, td, clusterName, clusterDomain, defaultParentIDTemplate)
	require.NoError(t, err)

	require.Equal(t, entry.ParentID.String(), fmt.Sprintf("spiffe://%s/spire/agent/x509pop/test.example.org", td))
}

func TestParseSPIFFEIDInput(t *testing.T) {
	td, err := spiffeid.TrustDomainFromString(trustDomain)
	require.NoError(t, err)

	t.Run("full URI", func(t *testing.T) {
		id, err := parseSPIFFEIDInput("spiffe://example.org/my-service", td)
		require.NoError(t, err)
		require.Equal(t, "spiffe://example.org/my-service", id.String())
	})

	t.Run("path only", func(t *testing.T) {
		id, err := parseSPIFFEIDInput("/my-service", td)
		require.NoError(t, err)
		require.Equal(t, "spiffe://example.org/my-service", id.String())
	})

	t.Run("invalid without leading slash", func(t *testing.T) {
		_, err := parseSPIFFEIDInput("my-service", td)
		require.Error(t, err)
	})

	t.Run("empty string", func(t *testing.T) {
		_, err := parseSPIFFEIDInput("", td)
		require.Error(t, err)
	})

	t.Run("invalid full URI", func(t *testing.T) {
		_, err := parseSPIFFEIDInput("spiffe://", td)
		require.Error(t, err)
	})

	t.Run("not a URI", func(t *testing.T) {
		_, err := parseSPIFFEIDInput("not-a-uri", td)
		require.Error(t, err)
	})

	t.Run("invalid path segment", func(t *testing.T) {
		_, err := parseSPIFFEIDInput("/bad/path/../segment", td)
		require.Error(t, err)
	})
}

func TestRenderStaticEntry(t *testing.T) {
	td, err := spiffeid.TrustDomainFromString(trustDomain)
	require.NoError(t, err)

	t.Run("full URI", func(t *testing.T) {
		spec := &spirev1alpha1.ClusterStaticEntrySpec{
			SPIFFEID:  "spiffe://example.org/static-spiffe-id",
			ParentID:  "spiffe://example.org/static-parent-id",
			Selectors: []string{"static:one"},
		}
		entry, err := renderStaticEntry(spec, td)
		require.NoError(t, err)
		require.Equal(t, "spiffe://example.org/static-spiffe-id", entry.SPIFFEID.String())
		require.Equal(t, "spiffe://example.org/static-parent-id", entry.ParentID.String())
	})

	t.Run("path only", func(t *testing.T) {
		spec := &spirev1alpha1.ClusterStaticEntrySpec{
			SPIFFEID:  "/static-spiffe-id",
			ParentID:  "/static-parent-id",
			Selectors: []string{"static:one"},
		}
		entry, err := renderStaticEntry(spec, td)
		require.NoError(t, err)
		require.Equal(t, "spiffe://example.org/static-spiffe-id", entry.SPIFFEID.String())
		require.Equal(t, "spiffe://example.org/static-parent-id", entry.ParentID.String())
	})

	t.Run("foreign trust domain in full URI", func(t *testing.T) {
		spec := &spirev1alpha1.ClusterStaticEntrySpec{
			SPIFFEID:  "spiffe://other.example.org/static-spiffe-id",
			ParentID:  "spiffe://other.example.org/static-parent-id",
			Selectors: []string{"static:one"},
		}
		entry, err := renderStaticEntry(spec, td)
		require.NoError(t, err)
		require.Equal(t, "spiffe://other.example.org/static-spiffe-id", entry.SPIFFEID.String())
		require.Equal(t, "spiffe://other.example.org/static-parent-id", entry.ParentID.String())
	})

	t.Run("empty spiffeID", func(t *testing.T) {
		spec := &spirev1alpha1.ClusterStaticEntrySpec{
			SPIFFEID:  "",
			ParentID:  "/static-parent-id",
			Selectors: []string{"static:one"},
		}
		_, err := renderStaticEntry(spec, td)
		require.Error(t, err)
		require.Contains(t, err.Error(), "SPIFFEID")
	})

	t.Run("empty parentID", func(t *testing.T) {
		spec := &spirev1alpha1.ClusterStaticEntrySpec{
			SPIFFEID:  "/static-spiffe-id",
			ParentID:  "",
			Selectors: []string{"static:one"},
		}
		_, err := renderStaticEntry(spec, td)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ParentID")
	})

	t.Run("invalid spiffeID with valid parentID", func(t *testing.T) {
		spec := &spirev1alpha1.ClusterStaticEntrySpec{
			SPIFFEID:  "bad-id",
			ParentID:  "/static-parent-id",
			Selectors: []string{"static:one"},
		}
		_, err := renderStaticEntry(spec, td)
		require.Error(t, err)
		require.Contains(t, err.Error(), "SPIFFEID")
	})

	t.Run("valid spiffeID with invalid parentID", func(t *testing.T) {
		spec := &spirev1alpha1.ClusterStaticEntrySpec{
			SPIFFEID:  "/static-spiffe-id",
			ParentID:  "spiffe://",
			Selectors: []string{"static:one"},
		}
		_, err := renderStaticEntry(spec, td)
		require.Error(t, err)
		require.Contains(t, err.Error(), "ParentID")
	})
}
