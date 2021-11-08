module github.com/spiffe/spire-controller-manager

go 1.16

require (
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/spiffe/go-spiffe/v2 v2.0.0-beta.8
	github.com/spiffe/spire-api-sdk v1.0.3-0.20210928174034-4735c1b6518e
	github.com/stretchr/testify v1.6.1
	google.golang.org/grpc v1.40.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	sigs.k8s.io/controller-runtime v0.8.3
)
