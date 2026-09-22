package httpserver

import (
	"address-intelligence-platform/internal/platform/apperror"
	"address-intelligence-platform/internal/platform/logging"
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime"
	"strings"
)

// Middleware keeps request correlation/access logging outside recovery so that
// recovered 500s appear as completed failures with the same request ID.
func Middleware(logger *slog.Logger, next http.Handler) http.Handler {
	return logging.HTTP(logger, recoverPanics(logger, next))
}
func recoverPanics(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracked := &commitWriter{ResponseWriter: w}
		defer func() {
			if failure := recover(); failure != nil {
				if failure == http.ErrAbortHandler {
					panic(http.ErrAbortHandler)
				}
				logger.ErrorContext(r.Context(), "HTTP handler panic", "error_code", "INTERNAL_ERROR", "stack_trace", stackTrace())
				// Never append JSON to an already committed or hijacked response.
				if tracked.committed {
					panic(http.ErrAbortHandler)
				}
				for key := range w.Header() {
					if key != "X-Request-Id" {
						w.Header().Del(key)
					}
				}
				WriteError(w, r, apperror.ErrInternal)
			}
		}()
		next.ServeHTTP(tracked, r)
	})
}

type commitWriter struct {
	http.ResponseWriter
	committed bool
}

func (w *commitWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *commitWriter) WriteHeader(status int) {
	if w.committed {
		return
	}
	w.ResponseWriter.WriteHeader(status)
	if status == http.StatusSwitchingProtocols || status >= 200 {
		w.committed = true
	}
}
func (w *commitWriter) Write(data []byte) (int, error) {
	if !w.committed {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}
func (w *commitWriter) FlushError() error {
	if !w.committed {
		w.WriteHeader(http.StatusOK)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}
func (w *commitWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, buffer, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err == nil {
		w.committed = true
	}
	return conn, buffer, err
}

// Use symbolized frames, without argument values or panic payloads.
func stackTrace() string {
	pcs := make([]uintptr, 64)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	var result strings.Builder
	for {
		frame, more := frames.Next()
		result.WriteString(frame.Function)
		result.WriteByte('\n')
		result.WriteString(frame.File)
		result.WriteByte(':')
		result.WriteString(fmt.Sprint(frame.Line))
		result.WriteByte('\n')
		if !more {
			break
		}
	}
	return result.String()
}
