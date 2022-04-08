package k8stest

import (
	spirev1alpha1 "github.com/spiffe/spire-controller-manager/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func NewClientBuilder() *fake.ClientBuilder {
	return WithScheme(fake.NewClientBuilder())
}

func WithScheme(b *fake.ClientBuilder) *fake.ClientBuilder {
	scheme := runtime.NewScheme()
	spirev1alpha1.AddToScheme(scheme)
	return b.WithScheme(scheme)
}
