/*
Copyright 2021 SPIRE Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package tracing initializes the global OpenTelemetry tracer provider and
// provides a thin accessor for call sites.
//
// When tracing is disabled (the default), a no-op TracerProvider is installed
// so that all otel.Tracer(...).Start(...) calls compile and run with zero
// overhead — no spans are created, allocated, or exported.
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// instrumentationName is the OTel instrumentation scope used for all spans
// produced by this controller.
const instrumentationName = "github.com/spiffe/spire-controller-manager"

// Config holds tracing configuration read from the controller manager config file.
type Config struct {
	// Enabled controls whether distributed tracing is active.
	// When false, a no-op TracerProvider is installed and there is zero overhead.
	Enabled bool

	// OTLPEndpoint is the OTLP/gRPC endpoint to export traces to
	// (e.g. "otel-collector:4317"). When empty the standard
	// OTEL_EXPORTER_OTLP_ENDPOINT environment variable is honoured.
	OTLPEndpoint string

	// SampleRatio is the fraction of traces to sample (0.0–1.0).
	// Zero or negative defaults to 1.0 (100 % sampling).
	SampleRatio float64

	// ClusterName is the Kubernetes cluster name, used to populate the
	// k8s.cluster.name resource attribute on every exported span.
	ClusterName string
}

// Init initialises the global OpenTelemetry tracer provider from cfg.
//
// When cfg.Enabled is false, a no-op provider is installed and shutdown is a
// no-op — this is the safe default. When enabled, an OTLP/gRPC exporter is
// created and wired to a batch trace processor.
//
// The returned shutdown function must be deferred by the caller; it flushes
// and closes the exporter gracefully.
func Init(ctx context.Context, cfg Config, serviceName, serviceVersion string) (shutdown func(context.Context) error, err error) {
	if !cfg.Enabled {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return func(context.Context) error { return nil }, nil
	}

	dialOpts := []otlptracegrpc.Option{otlptracegrpc.WithInsecure()}
	if cfg.OTLPEndpoint != "" {
		dialOpts = append(dialOpts, otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint))
	}

	exporter, err := otlptracegrpc.New(ctx, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP trace exporter: %w", err)
	}

	resAttrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String(serviceVersion),
	}
	if cfg.ClusterName != "" {
		resAttrs = append(resAttrs, semconv.K8SClusterNameKey.String(cfg.ClusterName))
	}

	// Detector order matters: later detectors override earlier ones.
	// WithFromEnv is listed first so that our explicit attributes (last) always win.
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithAttributes(resAttrs...),
	)
	if err != nil {
		// resource.New returns a partial resource + a joined error when some
		// detectors fail (e.g. process info unavailable in a container). Keep
		// the partial result rather than discarding all attributes.
		_ = err
	}

	ratio := cfg.SampleRatio
	if ratio <= 0 {
		ratio = 1.0
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(ratio)),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// Tracer returns the global tracer for the spire-controller-manager instrumentation scope.
// When tracing is disabled this returns the no-op tracer installed by Init.
func Tracer() trace.Tracer {
	return otel.Tracer(instrumentationName)
}
