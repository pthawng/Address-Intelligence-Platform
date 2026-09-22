package main

import (
	"address-intelligence-platform/internal/bootstrap"
	"context"
	"os"
	"os/signal"
	"syscall"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx)
	stop()
	if err != nil {
		bootstrap.LogFailure("migrate", version, err)
		os.Exit(1)
	}
}
func run(ctx context.Context) error {
	command := ""
	if len(os.Args) == 2 {
		command = os.Args[1]
	}
	return bootstrap.RunMigrate(ctx, command)
}
