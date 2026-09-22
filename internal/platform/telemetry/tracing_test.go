package telemetry

import (
	"address-intelligence-platform/internal/platform/logging"
	"bytes"
	"context"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTracingCorrelationAndPrivacy(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	defer provider.Shutdown(context.Background())
	var output bytes.Buffer
	logger := logging.New(logging.Options{Output: &output})
	h := logging.RequestIDMiddleware(HTTPTracing(provider)(logging.Access(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !trace.SpanFromContext(r.Context()).SpanContext().IsValid() {
			t.Error("missing span")
		}
		w.WriteHeader(500)
	}))))
	r := httptest.NewRequest("GET", "/private-address?token=secret", nil)
	r.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatal("missing span")
	}
	if spans[0].SpanContext().TraceID().String() != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatal("propagation lost")
	}
	if strings.Contains(output.String(), "secret") || !strings.Contains(output.String(), "trace_id") || !strings.Contains(output.String(), "request_id") {
		t.Fatal(output.String())
	}
	for _, a := range spans[0].Attributes() {
		if strings.Contains(a.Value.AsString(), "private-address") {
			t.Fatal("raw path collected")
		}
	}
}
