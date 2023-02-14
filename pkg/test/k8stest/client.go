package k8stest

import (
	"testing"

	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func NewClientBuilder(t *testing.T) *fake.ClientBuilder {
	return WithScheme(t, fake.NewClientBuilder())
}

func WithScheme(t *testing.T, b *fake.ClientBuilder) *fake.ClientBuilder {
	scheme := runtime.NewScheme()
	err := spirev1alpha1.AddToScheme(scheme)
	require.NoError(t, err)
	return b.WithScheme(scheme)
}
