package httpserver

import (
	"address-intelligence-platform/internal/platform/logging"
	"encoding/json"
	"net/http"
	"time"
)

// Timeout uses net/http's synchronized response buffer. It cancels context and
// returns 503 without allowing late handler writes to corrupt the response.
// Handlers must honor cancellation. Streaming/hijacking is not supported.
func Timeout(duration time.Duration, next http.Handler) http.Handler {
	if duration <= 0 {
		panic("HTTP handler timeout must be positive")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := json.Marshal(ErrorResponse{Status: 503, Error: ErrorDetail{Code: "REQUEST_TIMEOUT", Message: "Request timed out", RequestID: logging.RequestID(r.Context())}})
		w.Header().Set("Cache-Control", "no-store")
		http.TimeoutHandler(next, duration, string(body)+"\n").ServeHTTP(&jsonTimeoutWriter{ResponseWriter: w, head: r.Method == http.MethodHead}, r)
	})
}

type jsonTimeoutWriter struct {
	http.ResponseWriter
	head bool
}

func (w *jsonTimeoutWriter) WriteHeader(status int) {
	if status == http.StatusServiceUnavailable && w.Header().Get("Content-Type") == "" || w.Header().Get("Content-Type") == "text/html; charset=utf-8" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *jsonTimeoutWriter) Write(p []byte) (int, error) {
	if w.head {
		return len(p), nil
	}
	return w.ResponseWriter.Write(p)
}
