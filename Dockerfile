# Build the manager binary
FROM golang:1.19.5-alpine as builder
WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.* ./
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
#FROM gcr.io/distroless/static:nonroot
FROM gcr.io/distroless/base
WORKDIR /
COPY --from=builder /workspace/manager .
#USER 65532:65532

ENTRYPOINT ["/manager"]
