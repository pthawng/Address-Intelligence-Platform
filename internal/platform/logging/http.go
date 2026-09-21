package logging

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

// HTTP emits one completion event. Incoming request IDs are intentionally not
// trusted; the server returns its own ID. Query strings and bodies are omitted.
func HTTP(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var id [16]byte
		_, _ = rand.Read(id[:]) // crypto/rand.Read cannot fail on supported Go versions.
		requestID := hex.EncodeToString(id[:])
		r = r.WithContext(WithRequestID(r.Context(), requestID))
		w.Header().Set("X-Request-ID", requestID)
		response := &responseWriter{ResponseWriter: w}
		start := time.Now()
		completed := false
		defer func() {
			status := response.status
			if status == 0 && completed {
				status = http.StatusOK
			}
			level := slog.LevelInfo
			attrs := []any{"method", r.Method, "path", r.URL.Path, "status_code", status,
				"duration_ms", float64(time.Since(start)) / float64(time.Millisecond)}
			switch {
			case !completed:
				level = slog.LevelError
				attrs = append(attrs, "error_code", "request_aborted")
			case status >= 500:
				level = slog.LevelError
				attrs = append(attrs, "error_code", "http_server_error")
			case status >= 400:
				level = slog.LevelWarn
				attrs = append(attrs, "error_code", "http_client_error")
			}
			logger.Log(r.Context(), level, "HTTP request completed", attrs...)
		}()
		next.ServeHTTP(response, r)
		completed = true
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

// Unwrap supports streaming/deadlines via http.ResponseController.
func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *responseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.ResponseWriter.WriteHeader(status)
	if status == http.StatusSwitchingProtocols || status >= 200 {
		w.status = status
	}
}
func (w *responseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}
func (w *responseWriter) FlushError() error {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}
