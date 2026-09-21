// Package bootstrap contains the composition roots for the application runtimes.
package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"address-intelligence-platform/internal/platform/config"
	platformlog "address-intelligence-platform/internal/platform/logging"
)

func RunAPI(ctx context.Context) error {
	cfg, err := config.LoadFor(config.API)
	if err != nil {
		return err
	}

	platformlog.Configure(cfg.LogLevel)
	slog.Info("api runtime initialized", "environment", cfg.Environment, "address", cfg.HTTPAddress)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", health)
	mux.HandleFunc("GET /health/ready", health)

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           mux,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

func health(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte("{\"status\":\"ok\"}\n"))
}

func RunIndexer(ctx context.Context) error {
	cfg, err := config.LoadFor(config.Indexer)
	if err != nil {
		return err
	}

	platformlog.Configure(cfg.LogLevel)
	slog.Info("indexer runtime initialized", "environment", cfg.Environment, "poll_interval", cfg.OutboxPollInterval)
	<-ctx.Done()
	return nil
}

func RunWorker(ctx context.Context) error {
	cfg, err := config.LoadFor(config.Worker)
	if err != nil {
		return err
	}

	platformlog.Configure(cfg.LogLevel)
	slog.Info("worker runtime initialized", "environment", cfg.Environment)
	<-ctx.Done()
	return nil
}
