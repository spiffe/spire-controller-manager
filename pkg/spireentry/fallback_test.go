package spireentry

import (
	"context"
	"strconv"
	"testing"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
)

func TestAdditiveClusterSPIFFEIDKeepsTheFallback(t *testing.T) {
	for name, tc := range map[string]struct {
		additive     bool
		wantFallback bool
	}{
		"additive":     {additive: true, wantFallback: true},
		"not additive": {additive: false, wantFallback: false},
	} {
		t.Run(name, func(t *testing.T) {
			declared := declaredSPIFFEIDs(t,
				clusterSPIFFEID("fallback", "spiffe://{{ .TrustDomain }}/fallback", true, nil),
				clusterSPIFFEID("beside", "spiffe://{{ .TrustDomain }}/beside", false,
					map[string]string{spirev1alpha1.AdditiveAnnotation: strconv.FormatBool(tc.additive)}),
			)
			require.Contains(t, declared, "spiffe://example.org/beside")
			if tc.wantFallback {
				require.Contains(t, declared, "spiffe://example.org/fallback")
			} else {
				require.NotContains(t, declared, "spiffe://example.org/fallback")
			}
		})
	}
}

func TestAdditiveAnnotationIsValidated(t *testing.T) {
	for name, tc := range map[string]struct {
		value    string
		fallback bool
		wantErr  string
	}{
		"additive":          {value: "true"},
		"not additive":      {value: "false", fallback: true},
		"additive fallback": {value: "true", fallback: true, wantErr: "fallback and"},
		"not a boolean":     {value: "yes", wantErr: `not "yes"`},
	} {
		t.Run(name, func(t *testing.T) {
			c := clusterSPIFFEID("c", "spiffe://{{ .TrustDomain }}/c", tc.fallback,
				map[string]string{spirev1alpha1.AdditiveAnnotation: tc.value})
			_, err := (&spirev1alpha1.ClusterSPIFFEIDCustomValidator{}).ValidateCreate(context.Background(), &c.ClusterSPIFFEID)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func clusterSPIFFEID(name, template string, fallback bool, annotations map[string]string) *ClusterSPIFFEID {
	return &ClusterSPIFFEID{ClusterSPIFFEID: spirev1alpha1.ClusterSPIFFEID{
		ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: annotations},
		Spec:       spirev1alpha1.ClusterSPIFFEIDSpec{SPIFFEIDTemplate: template, Fallback: fallback},
	}}
}

// declaredSPIFFEIDs renders the given ClusterSPIFFEIDs for one scheduled pod
// and returns the SPIFFE IDs they declare for it.
func declaredSPIFFEIDs(t *testing.T, clusterSPIFFEIDs ...*ClusterSPIFFEID) []string {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-a", UID: "node-uid"}}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "default", UID: "pod-uid"},
			Spec:       corev1.PodSpec{NodeName: node.Name},
		},
		node,
	).Build()

	reconciler := &entryReconciler{config: ReconcilerConfig{
		TrustDomain: spiffeid.RequireTrustDomainFromString("example.org"),
		ClusterName: "cluster",
		K8sClient:   k8sClient,
	}}
	state := make(entriesState)
	reconciler.addClusterSPIFFEIDEntriesState(context.Background(), state, clusterSPIFFEIDs, map[string]*corev1.Node{node.Name: node})

	var declared []string
	for _, entry := range state {
		for _, d := range entry.Declared {
			declared = append(declared, d.Entry.SPIFFEID.String())
		}
	}
	return declared
}
