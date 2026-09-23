package bootstrap

import (
	"context"
	"errors"

	sourceimport "address-intelligence-platform/internal/importjob/infrastructure/postgres"
	"address-intelligence-platform/internal/platform/config"
)

// RunImportData uses migration credentials and never runs during API startup.
func RunImportData(ctx context.Context, dir string) (int, bool, error) {
	url, err := config.MigrationURL()
	if err != nil {
		return 0, false, err
	}
	if url == "" {
		return 0, false, errors.New("migration database credentials are required")
	}
	return sourceimport.Import(ctx, url, dir)
}
