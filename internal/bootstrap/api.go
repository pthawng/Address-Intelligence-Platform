package bootstrap

import (
	"address-intelligence-platform/internal/platform/telemetry"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"address-intelligence-platform/internal/platform/config"
	"address-intelligence-platform/internal/platform/database"
	"address-intelligence-platform/internal/platform/httpserver"
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
		metrics := new(httpserver.Metrics)
		probe := func(ctx context.Context) error { return database.Ready(ctx, a.db) }
		handler := databaseHealthRoutes(probe, a.cfg.DatabasePool.ProbeTimeout)
		if a.cfg.Environment == "development" || a.cfg.Environment == "test" {
			handler = databaseHealthRoutes(probe, a.cfg.DatabasePool.ProbeTimeout, metrics)
		}
		return a.serveHTTP(ctx, owned, listener, handler, metrics)
	}
	return a, nil
}

func (a *App) serveHTTP(ctx context.Context, owned *resources, listener net.Listener, handler http.Handler, observations ...*httpserver.Metrics) error {
	tracing, shutdownTracing, err := telemetry.NewTracing(ctx, a.cfg.OTLPEndpoint)
	if err != nil {
		return err
	}
	owned.add("tracing", shutdownTracing)
	metrics := new(httpserver.Metrics)
	if len(observations) > 0 {
		metrics = observations[0]
	}
	var origins []string
	for _, origin := range strings.Split(a.cfg.CORSOrigins, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			origins = append(origins, origin)
		}
	}
	// Existing requests survive the stop signal during graceful draining.
	requests, cancelRequests := context.WithCancel(context.WithoutCancel(ctx))
	server, err := httpserver.New(requests, httpserver.Config{
		ReadHeaderTimeout: a.cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       a.cfg.HTTPReadTimeout,
		WriteTimeout:      a.cfg.HTTPWriteTimeout,
		IdleTimeout:       a.cfg.HTTPIdleTimeout,
		MaxHeaderBytes:    a.cfg.HTTPMaxHeaderBytes,
	}, handler, a.logger, httpserver.MiddlewareOptions{
		Origins: origins, Timeout: a.cfg.HTTPRequestTimeout, RatePerSecond: a.cfg.HTTPRatePerSecond, Burst: a.cfg.HTTPRateBurst, Tracing: tracing, Metrics: metrics,
	})
	if err != nil {
		cancelRequests()
		return err
	}

	done := make(chan struct{})
	var serveErr error // read only after done closes
	owned.add("HTTP server", func(ctx context.Context) error {
		defer cancelRequests()
		shutdownErr := server.Shutdown(ctx)
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
