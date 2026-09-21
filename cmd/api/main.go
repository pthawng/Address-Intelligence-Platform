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
		bootstrap.LogFailure("api", version, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		return checkHealth()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.NewAPI(bootstrap.WithVersion(version))
	if err != nil {
		return err
	}
	return app.Run(ctx)
}
