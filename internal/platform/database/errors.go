package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/pressly/goose/v3"
)

var (
	errLegacyDetected = errors.New("legacy database detected; verify backup and use adopt-legacy")
	errLegacyRequired = errors.New("adopt-legacy requires an existing legacy schema")
	errLegacyLedger   = errors.New("legacy version ledger is not the expected baseline")
	errLegacyDrift    = errors.New("legacy schema drift detected; adoption refused")
)

// SafeMigrationError keeps actionable metadata without SQL, credentials, row
// values or server diagnostics, which Goose's raw errors can contain.
func SafeMigrationError(err error) error {
	if err == nil {
		return nil
	}
	message := "migration failed"
	var partial *goose.PartialError
	if errors.As(err, &partial) && partial.Failed != nil {
		message += fmt.Sprintf(" at version %d", partial.Failed.Source.Version)
	}
	var state interface{ SQLState() string }
	if errors.As(err, &state) {
		message += fmt.Sprintf(" (SQLSTATE %s)", state.SQLState())
	}
	for _, known := range []error{errLegacyDetected, errLegacyRequired, errLegacyLedger, errLegacyDrift, context.Canceled, context.DeadlineExceeded} {
		if errors.Is(err, known) {
			return fmt.Errorf("%s: %w", message, known)
		}
	}
	return errors.New(message + "; check database server diagnostics and migration status")
}
