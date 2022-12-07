package spireentry

import (
	"testing"

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
			"{{ .PodSpec.ServiceAccountName }}.{{ .PodMeta.Namespace }}.svc",
			"{{ .PodMeta.Name }}.{{ .PodMeta.Namespace }}.svc",
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

	parsedSpec, err := spirev1alpha1.ParseClusterSPIFFEIDSpec(spec)
	require.NoError(t, err)
	td, err := spiffeid.TrustDomainFromString(trustDomain)
	require.NoError(t, err)

	entry, err := renderPodEntry(parsedSpec, node, pod, td, clusterName, clusterDomain)
	require.NoError(t, err)

	// SPIFFE ID rendered correctly
	spiffeID, err := spiffeid.FromPathf(td, "/ns/%s/sa/%s", pod.Namespace, pod.Spec.ServiceAccountName)
	require.NoError(t, err)
	require.Equal(t, entry.SPIFFEID.String(), spiffeID.String())

	// Parent ID rendered correctly
	parentID, err := spiffeid.FromPathf(td, "/spire/agent/k8s_psat/%s/%s", clusterName, node.UID)
	require.NoError(t, err)
	require.Equal(t, entry.ParentID.String(), parentID.String())

	// DNS names rendered correctly and are unique
	require.Len(t, entry.DNSNames, len(spec.DNSNameTemplates)-1)
	require.Contains(t, entry.DNSNames, pod.Name+"."+pod.Namespace+"."+"svc")
	require.Contains(t, entry.DNSNames, pod.Name+"."+trustDomain+"."+"svc")
}
