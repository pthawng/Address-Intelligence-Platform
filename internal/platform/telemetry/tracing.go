package telemetry

import (
	"address-intelligence-platform/internal/platform/logging"
	"context"
	"fmt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"net/http"
)

// NewTracing owns an instance-local provider; globals are never changed.
// Empty endpoint still creates trace IDs, but does not export spans.
func NewTracing(ctx context.Context, endpoint string) (func(http.Handler) http.Handler, func(context.Context) error, error) {
	options := []sdktrace.TracerProviderOption{sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", "address-intelligence-platform-api"))), sdktrace.WithSampler(sdktrace.TraceIDRatioBased(0.1))}
	if endpoint != "" {
		exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(endpoint))
		if err != nil {
			return nil, nil, fmt.Errorf("initialize OTLP tracing: %w", err)
		}
		options = append(options, sdktrace.WithBatcher(exporter))
	}
	provider := sdktrace.NewTracerProvider(options...)
	return HTTPTracing(provider), provider.Shutdown, nil
}

func HTTPTracing(provider trace.TracerProvider) func(http.Handler) http.Handler {
	tracer := provider.Tracer("address-intelligence-platform/http")
	propagator := propagation.TraceContext{}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			// Do not collect raw URL paths, queries, headers or user identifiers.
			ctx, span := tracer.Start(ctx, "HTTP request", trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()
			ctx = logging.WithTraceID(ctx, span.SpanContext().TraceID().String())
			response := &traceWriter{ResponseWriter: w}
			completed := false
			defer func() {
				status := response.status
				if status == 0 && completed {
					status = 200
				}
				span.SetAttributes(attribute.Int("http.response.status_code", status))
				if !completed || status >= 500 {
					span.SetStatus(codes.Error, "request failed")
				}
			}()
			next.ServeHTTP(response, r.WithContext(ctx))
			completed = true
		})
	}
}

type traceWriter struct {
	http.ResponseWriter
	status int
}

func (w *traceWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *traceWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.ResponseWriter.WriteHeader(status)
	if status >= 200 || status == 101 {
		w.status = status
	}
}
func (w *traceWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(p)
}
func (w *traceWriter) FlushError() error {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}
