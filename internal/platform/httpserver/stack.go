package httpserver

import (
	"address-intelligence-platform/internal/platform/logging"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type MiddlewareOptions struct {
	Origins       []string
	Timeout       time.Duration
	RatePerSecond int
	Burst         int
	Tracing       func(http.Handler) http.Handler
	Metrics       *Metrics
}

func stack(logger *slog.Logger, next http.Handler, o MiddlewareOptions) (http.Handler, error) {
	if o.Timeout <= 0 {
		return nil, fmt.Errorf("handler timeout must be positive")
	}
	limiter, err := NewLimiter(o.RatePerSecond, o.Burst)
	if err != nil {
		return nil, err
	}
	limited := limiter.Wrap(next)
	// Probe endpoints remain available even when the aggregate budget is exhausted.
	dispatch := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (r.Method == "GET" || r.Method == "HEAD") && (r.URL.Path == "/health/live" || r.URL.Path == "/health/ready" || r.URL.Path == "/metrics") {
			next.ServeHTTP(w, r)
			return
		}
		limited.ServeHTTP(w, r)
	})
	// Recovery is inside TimeoutHandler's goroutine so panics are logged even
	// after the timeout response has been returned; outer recovery covers wrappers.
	timed := Timeout(o.Timeout, recoverPanics(logger, dispatch))
	cors, err := CORS(o.Origins, timed)
	if err != nil {
		return nil, err
	}
	var handler http.Handler = recoverPanics(logger, cors)
	if o.Metrics != nil {
		handler = o.Metrics.Wrap(handler)
	}
	handler = logging.Access(logger, handler)
	if o.Tracing != nil {
		handler = o.Tracing(handler)
	}
	return logging.RequestIDMiddleware(handler), nil
}
