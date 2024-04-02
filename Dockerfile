# Build the manager binary
FROM --platform=${BUILDPLATFORM} golang:1.22.1-alpine as base
WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.* ./
# Cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/
COPY pkg/ pkg/

# xx is a helper for cross-compilation
# when bumping to a new version analyze the new version for security issues
# then use crane to lookup the digest of that version so we are immutable
# crane digest tonistiigi/xx:1.3.0
FROM --platform=${BUILDPLATFORM} tonistiigi/xx@sha256:904fe94f236d36d65aeb5a2462f88f2c537b8360475f6342e7599194f291fb7e AS xx

# Build
FROM --platform=${BUILDPLATFORM} base as builder
ARG TARGETPLATFORM
ARG TARGETARCH
ENV CGO_ENABLED=0
COPY --link --from=xx / /
RUN xx-go --wrap
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -o bin/spire-controller-manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
#FROM gcr.io/distroless/static:nonroot
FROM gcr.io/distroless/static AS spire-controller-manager
WORKDIR /
ENTRYPOINT ["/spire-controller-manager"]
CMD []
COPY --link --from=builder /workspace/bin/spire-controller-manager /spire-controller-manager
#USER 65532:65532
