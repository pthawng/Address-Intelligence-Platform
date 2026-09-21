package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func decode(t *testing.T, b *bytes.Buffer) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(b.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestIdentityCorrelationAndFiltering(t *testing.T) {
	var b bytes.Buffer
	l := New(Options{Service: "api", Environment: "test", Version: "v1", Output: &b})
	ctx := WithTraceID(WithRequestID(context.Background(), "req1"), "trace1")
	l.DebugContext(ctx, "hidden")
	if b.Len() != 0 {
		t.Fatal("debug should be filtered")
	}
	l.With("component", "search").InfoContext(ctx, "searched")
	r := decode(t, &b)
	for k, want := range map[string]string{"service": "api", "environment": "test", "version": "v1", "request_id": "req1", "trace_id": "trace1", "component": "search"} {
		if r[k] != want {
			t.Errorf("%s = %v, want %s", k, r[k], want)
		}
	}
	b.Reset()
	l.WithGroup("search").With("provider", "local").InfoContext(ctx, "grouped", "count", 1)
	r = decode(t, &b)
	if r["request_id"] != "req1" || r["trace_id"] != "trace1" || r["service"] != "api" {
		t.Fatal("identity must stay at root", r)
	}
	if group, ok := r["search"].(map[string]any); !ok || group["provider"] != "local" || group["count"] != float64(1) {
		t.Fatal(r)
	}
	b.Reset()
	l.Info("startup")
	if r := decode(t, &b); r["request_id"] != nil || r["trace_id"] != nil {
		t.Fatal("correlation leaked")
	}
}

func TestHTTPCompletion(t *testing.T) {
	for _, status := range []int{200, 204, 404, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var b bytes.Buffer
			l := New(Options{Service: "api", Output: &b})
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/search?query=secret-address", nil)
			r.Header.Set("X-Request-ID", "untrusted")
			HTTP(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if status != 200 {
					w.WriteHeader(status)
					w.WriteHeader(201)
				}
			})).ServeHTTP(w, r)
			row := decode(t, &b)
			if row["status_code"] != float64(status) || row["path"] != "/search" || row["method"] != "GET" {
				t.Fatal(row)
			}
			id := w.Header().Get("X-Request-ID")
			if len(id) != 32 || row["request_id"] != id {
				t.Fatal("missing request correlation", row)
			}
			if strings.Contains(b.String(), "secret-address") || strings.Contains(b.String(), "untrusted") {
				t.Fatal("untrusted data logged")
			}
			if _, ok := row["duration_ms"].(float64); !ok {
				t.Fatal("duration must be numeric")
			}
			wantLevel := "INFO"
			if status >= 500 {
				wantLevel = "ERROR"
			} else if status >= 400 {
				wantLevel = "WARN"
			}
			if row["level"] != wantLevel {
				t.Fatal(row)
			}
		})
	}
}

func TestHTTPPanicPreservesServerRecovery(t *testing.T) {
	var b bytes.Buffer
	l := New(Options{Output: &b})
	func() {
		defer func() {
			if recover() != http.ErrAbortHandler {
				t.Fatal("panic swallowed")
			}
		}()
		HTTP(l, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	r := decode(t, &b)
	if r["status_code"] != float64(0) || r["error_code"] != "request_aborted" {
		t.Fatal(r)
	}
}

func TestResponseControllerAndInformationalStatus(t *testing.T) {
	w := &responseWriter{ResponseWriter: httptest.NewRecorder()}
	w.WriteHeader(103)
	if w.status != 0 {
		t.Fatal("informational status counted as final")
	}
	if err := http.NewResponseController(w).Flush(); err != nil {
		t.Fatal(err)
	}
	if w.status != 200 {
		t.Fatal(w.status)
	}
}

var _ slog.Handler = (*contextHandler)(nil)
