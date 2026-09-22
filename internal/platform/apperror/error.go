// Package apperror defines transport-independent error identities shared by core layers.
package apperror

import "errors"

// Code is an immutable sentinel and stable machine-readable identifier.
// Compare with errors.Is, never with error text.
type Code string

const (
	ErrAddressNotFound     Code = "ADDRESS_NOT_FOUND"
	ErrInvalidAddress      Code = "INVALID_ADDRESS"
	ErrInvalidCoordinates  Code = "INVALID_COORDINATES"
	ErrProviderUnavailable Code = "PROVIDER_UNAVAILABLE"
	ErrRateLimitExceeded   Code = "RATE_LIMIT_EXCEEDED"
	ErrInternal            Code = "INTERNAL_ERROR"
	ErrInvalidInput        Code = "INVALID_INPUT"
	ErrUnauthorized        Code = "UNAUTHORIZED"
	ErrForbidden           Code = "FORBIDDEN"
)

func (c Code) Error() string   { return string(c) }
func (c Code) ErrorCode() Code { return c }

// AppError classifies a cause without discarding it. Error text is diagnostic
// only; transports must use their own public message catalog.
type AppError struct {
	Code Code
	Err  error
}

func (e *AppError) Error() string {
	if e == nil {
		return string(ErrInternal)
	}
	if e.Err == nil {
		return e.Code.Error()
	}
	return e.Code.Error() + ": " + e.Err.Error()
}
func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
func (e *AppError) Is(target error) bool {
	code, ok := target.(Code)
	return e != nil && ok && e.Code == code
}
func (e *AppError) ErrorCode() Code {
	if e == nil {
		return ErrInternal
	}
	return e.Code
}

// CodeOf returns the outermost classification. With errors.Join the first
// classification in depth-first order wins. Unknown codes are left to the mapper.
func CodeOf(err error) Code {
	if err == nil {
		return ""
	}
	var classified interface {
		error
		ErrorCode() Code
	}
	if errors.As(err, &classified) {
		return classified.ErrorCode()
	}
	return ErrInternal
}

// FieldViolation contains a public field name and safe message, never a rejected
// value or raw validator/database error. Use snake_case API field paths.
type FieldViolation struct {
	Field   string
	Message string
}

type ValidationError struct{ Details []FieldViolation }

func (e *ValidationError) Error() string { return ErrInvalidInput.Error() }
func (e *ValidationError) ErrorCode() Code {
	if e == nil {
		return ErrInternal
	}
	return ErrInvalidInput
}
func (e *ValidationError) Is(target error) bool { return e != nil && target == ErrInvalidInput }
