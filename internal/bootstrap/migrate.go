package bootstrap

import (
	"address-intelligence-platform/internal/platform/config"
	"address-intelligence-platform/internal/platform/database"
	"address-intelligence-platform/internal/platform/logging"
	"context"
	"errors"
	"os"
	"time"
)

// RunMigrate never runs from an application runtime. Credentials stay in env.
func RunMigrate(ctx context.Context, command string) error {
	if command != "up" && command != "status" && command != "adopt-legacy" {
		return errors.New("usage: migrate up|status|adopt-legacy")
	}
	url, err := config.MigrationURL()
	if err != nil {
		return err
	}
	if url == "" {
		return errors.New("MIGRATION_DATABASE_URL or split MIGRATION_DATABASE_* settings are required")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	provider, err := database.NewMigrator(ctx, url, command == "adopt-legacy")
	if err != nil {
		return err
	}
	defer provider.Close()
	logger := logging.New(logging.Options{Service: "migrate", Environment: os.Getenv("APP_ENV")})
	if command == "status" {
		statuses, statusErr := provider.Status(ctx)
		err = statusErr
		for _, status := range statuses {
			logger.InfoContext(ctx, "migration status", "migration_version", status.Source.Version, "state", status.State)
		}
	} else {
		results, upErr := provider.Up(ctx)
		err = upErr
		if err == nil {
			logger.InfoContext(ctx, "migrations completed", "applied", len(results), "schema_version", database.SchemaVersion)
		}
	}
	if err != nil {
		return database.SafeMigrationError(err)
	}
	return nil
}
