package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"address-intelligence-platform/internal/bootstrap"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	if err := run(); err != nil {
		bootstrap.LogFailure("worker", version, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.NewWorker(bootstrap.WithVersion(version))
	if err != nil {
		return err
	}
	return app.Run(ctx)
}
