module github.com/spiffe/spire-controller-manager

go 1.16

require (
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/spiffe/go-spiffe/v2 v2.0.0-beta.10
	github.com/spiffe/spire-api-sdk v1.1.0
	github.com/stretchr/testify v1.7.0
	google.golang.org/grpc v1.40.0
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/controller-runtime v0.10.2
)
