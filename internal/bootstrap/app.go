// Package bootstrap owns construction, execution and cleanup of each runtime.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"

	"address-intelligence-platform/internal/platform/config"
	platformlog "address-intelligence-platform/internal/platform/logging"
)

// App is a single-use runtime. New constructors allocate no external resources;
// Run owns resources from acquisition through cleanup, including startup failures.
type App struct {
	cfg     config.Config
	logger  *slog.Logger
	started atomic.Bool
	run     func(context.Context, *resources) error
}

// Option configures runtime construction.
type Option func(*platformlog.Options)

func WithVersion(version string) Option {
	return func(opts *platformlog.Options) { opts.Version = version }
}

// LogFailure records terminal failures once, including configuration failures.
func LogFailure(service, version string, err error) { platformlog.Failure(service, version, err) }

func newApp(runtime config.Runtime, options ...Option) (*App, error) {
	cfg, err := config.LoadFor(runtime)
	if err != nil {
		return nil, fmt.Errorf("configure %s: %w", runtime, err)
	}
	opts := platformlog.Options{Service: string(runtime), Environment: cfg.Environment, Level: cfg.LogLevel}
	for _, option := range options {
		option(&opts)
	}
	return &App{cfg: cfg, logger: platformlog.New(opts)}, nil
}

// Run blocks until cancellation or failure. Cancellation is a normal stop;
// startup, serving and cleanup failures remain errors. Do not call Run twice.
func (a *App) Run(ctx context.Context) (err error) {
	if a.run == nil {
		return errors.New("bootstrap: application is not initialized")
	}
	if !a.started.CompareAndSwap(false, true) {
		return errors.New("bootstrap: application already ran")
	}
	if ctx.Err() != nil {
		return nil
	}
	var owned resources
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.cfg.ShutdownTimeout)
		defer cancel()
		err = errors.Join(err, owned.close(shutdownCtx))
		if err == nil {
			a.logger.Info("runtime stopped")
		}
	}()
	err = a.run(ctx, &owned)
	return err
}
