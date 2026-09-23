package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"address-intelligence-platform/internal/bootstrap"
)

func main() {
	dir := flag.String("dir", "docs/data", "directory containing the six SuperShip SQL snapshots")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	count, unchanged, err := bootstrap.RunImportData(ctx, *dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "source import failed:", err)
		os.Exit(1)
	}
	if unchanged {
		fmt.Println("source snapshots already imported; no changes")
	} else {
		fmt.Printf("imported six source snapshots; promoted %d administrative units; level 4 retained unresolved\n", count)
	}
}
