package main

import (
	"context"
	"log/slog"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := bootstrap.RunIndexer(ctx); err != nil {
		slog.Error("indexer stopped", "error", err)
		os.Exit(1)
	}
}
