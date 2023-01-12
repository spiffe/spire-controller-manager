# Build the manager binary
FROM --platform=${BUILDPLATFORM} golang:1.19.5-alpine as base
WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.* ./
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/

# xx is a helper for cross-compilation
# when bumping to a new version analyze the new version for security issues
# then use crane to lookup the digest of that version so we are immutable
# crane digest tonistiigi/xx:1.1.2
FROM --platform=${BUILDPLATFORM} tonistiigi/xx@sha256:9dde7edeb9e4a957ce78be9f8c0fbabe0129bf5126933cd3574888f443731cda AS xx

# Build
FROM --platform=${BUILDPLATFORM} base as builder
ARG TARGETPLATFORM
ARG TARGETARCH
ENV CGO_ENABLED=0
COPY --link --from=xx / /
RUN xx-go --wrap
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -ldflags '-s -w' -o bin/spire-controller-manager main.go

# See https://github.com/chainguard-images/images/tree/main/images/static
# Used to run static binaries, includes tzdata and ca-certificates
FROM cgr.dev/chainguard/static:latest AS spire-controller-manager
WORKDIR /
ENTRYPOINT ["/spire-controller-manager"]
CMD []
COPY --link --from=builder /workspace/bin/spire-controller-manager /spire-controller-manager
#USER 65532:65532
