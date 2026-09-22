// Package httpserver provides the shared HTTP transport foundation.
package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Config must contain explicit positive limits; zero never disables protection.
type Config struct {
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	MaxHeaderBytes    int
}

// New configures but does not start a server. Bootstrap owns its listener and
// shutdown deadline. ctx must outlive graceful request draining.
func New(ctx context.Context, cfg Config, handler http.Handler, logger *slog.Logger, options ...MiddlewareOptions) (*http.Server, error) {
	if cfg.ReadTimeout <= 0 || cfg.WriteTimeout <= 0 || cfg.IdleTimeout <= 0 || cfg.ReadHeaderTimeout <= 0 || cfg.MaxHeaderBytes <= 0 {
		return nil, fmt.Errorf("HTTP timeouts and MaxHeaderBytes must be positive")
	}
	if handler == nil || logger == nil || ctx == nil {
		return nil, fmt.Errorf("HTTP handler, logger and base context are required")
	}
	wrapped := Middleware(logger, handler)
	if len(options) > 1 {
		return nil, fmt.Errorf("only one middleware options value is supported")
	}
	if len(options) == 1 {
		if options[0].Timeout >= cfg.WriteTimeout {
			return nil, fmt.Errorf("handler timeout must be shorter than WriteTimeout")
		}
		var err error
		wrapped, err = stack(logger, handler, options[0])
		if err != nil {
			return nil, err
		}
	}
	return &http.Server{
		Handler:     wrapped,
		ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout,
		IdleTimeout: cfg.IdleTimeout, ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		MaxHeaderBytes: cfg.MaxHeaderBytes,
		BaseContext:    func(net.Listener) context.Context { return ctx },
		ErrorLog:       slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}, nil
}
