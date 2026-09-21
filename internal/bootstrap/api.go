package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"address-intelligence-platform/internal/platform/config"
	"address-intelligence-platform/internal/platform/logging"
)

func NewAPI(options ...Option) (*App, error) {
	a, err := newApp(config.API, options...)
	if err != nil {
		return nil, err
	}
	a.run = func(ctx context.Context, owned *resources) error {
		listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", a.cfg.HTTPAddress)
		if err != nil {
			if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
				return nil
			}
			return fmt.Errorf("listen HTTP: %w", err)
		}
		owned.add("HTTP listener", func(context.Context) error {
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				return err
			}
			return nil
		})
		return a.serveHTTP(ctx, owned, listener, healthRoutes())
	}
	return a, nil
}

func (a *App) serveHTTP(ctx context.Context, owned *resources, listener net.Listener, handler http.Handler) error {
	// Existing requests survive the stop signal during graceful draining.
	requests, cancelRequests := context.WithCancel(context.WithoutCancel(ctx))
	server := &http.Server{
		Handler:           logging.HTTP(a.logger, handler),
		ReadHeaderTimeout: a.cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       a.cfg.HTTPReadTimeout,
		WriteTimeout:      a.cfg.HTTPWriteTimeout,
		IdleTimeout:       a.cfg.HTTPIdleTimeout,
		BaseContext:       func(net.Listener) context.Context { return requests },
		ErrorLog:          slog.NewLogLogger(a.logger.Handler(), slog.LevelError),
	}
	done := make(chan struct{})
	var serveErr error // read only after done closes
	owned.add("HTTP server", func(shutdownCtx context.Context) error {
		defer cancelRequests()
		shutdownErr := server.Shutdown(shutdownCtx)
		if shutdownErr != nil {
			cancelRequests()
			shutdownErr = errors.Join(shutdownErr, server.Close())
		}
		<-done
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return errors.Join(shutdownErr, serveErr)
	})
	go func() {
		serveErr = server.Serve(listener)
		close(done)
	}()
	a.logger.Info("HTTP server listening", "address", listener.Addr().String())
	select {
	case <-ctx.Done():
	case <-done:
	}
	// The registered closer drains requests and reports both serving and shutdown
	// errors. It executes before dependency closers registered earlier in startup.
	return nil
}

// RunAPI is the convenience entry point for callers that do not need App access.
func RunAPI(ctx context.Context) error {
	a, err := NewAPI()
	if err != nil {
		return err
	}
	return a.Run(ctx)
}
