// Package logging configures process-wide structured logging.
package logging

import (
	"log/slog"
	"os"
)

func Configure(level slog.Level) {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}
