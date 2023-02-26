FROM golang:1.20.1-alpine AS builder
WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download
COPY cmd/ cmd/
RUN CGO_ENABLED=0 go build ./cmd/greeter-client
RUN CGO_ENABLED=0 go build ./cmd/greeter-server

#FROM gcr.io/distroless/static:nonroot AS greeter-client
FROM alpine AS greeter-client
WORKDIR /
COPY --from=builder /workspace/greeter-client .
USER 65532:65532
ENTRYPOINT ["/greeter-client"]

#FROM gcr.io/distroless/static:nonroot AS greeter-server
FROM alpine AS greeter-server
WORKDIR /
COPY --from=builder /workspace/greeter-server .
USER 65532:65532
ENTRYPOINT ["/greeter-server"]
