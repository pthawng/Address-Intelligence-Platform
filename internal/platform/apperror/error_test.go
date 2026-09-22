package apperror

import (
	"errors"
	"fmt"
	"testing"
)

func TestClassificationAndCause(t *testing.T) {
	cause := errors.New("private provider detail")
	app := &AppError{Code: ErrProviderUnavailable, Err: cause}
	wrapped := fmt.Errorf("resolve: %w", app)
	if !errors.Is(wrapped, ErrProviderUnavailable) || !errors.Is(wrapped, cause) {
		t.Fatal("lost identity or cause")
	}
	var got *AppError
	if !errors.As(wrapped, &got) || got != app {
		t.Fatal("lost typed error")
	}
	if CodeOf(wrapped) != ErrProviderUnavailable {
		t.Fatal("lost classification")
	}
	if CodeOf(&AppError{Code: ErrInternal, Err: ErrInvalidAddress}) != ErrInternal {
		t.Fatal("inner error overrode outer classification")
	}
	if CodeOf(errors.Join(ErrInvalidCoordinates, ErrInternal)) != ErrInvalidCoordinates {
		t.Fatal("join ordering changed")
	}
	if CodeOf(nil) != "" || CodeOf(cause) != ErrInternal {
		t.Fatal("unexpected default")
	}
	var typedNil *AppError
	if CodeOf(typedNil) != ErrInternal {
		t.Fatal("typed nil must fail closed")
	}
}
