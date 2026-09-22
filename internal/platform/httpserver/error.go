// Package httpserver owns the public HTTP error contract.
package httpserver

import (
	"address-intelligence-platform/internal/platform/apperror"
	"address-intelligence-platform/internal/platform/logging"
	"encoding/json"
	"errors"
	"net/http"
)

type ErrorResponse struct {
	Success bool        `json:"success"`
	Status  int         `json:"status"`
	Error   ErrorDetail `json:"error"`
}
type ErrorDetail struct {
	Code      string       `json:"code"`
	Details   []FieldError `json:"details,omitempty"`
	Message   string       `json:"message"`
	RequestID string       `json:"request_id,omitempty"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Handler returns errors before committing headers or writing a body.
// Successful responses remain the endpoint's responsibility.
type Handler func(http.ResponseWriter, *http.Request) error

func Adapt(handler Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			WriteError(w, r, err)
		}
	}
}

// WriteError never serializes err.Error() or the underlying cause.
// Nil is a no-op. Callers must not have committed a response.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}
	status, code, message := classify(err)
	var details []FieldError
	// Inspect only the outermost classification, not validation hidden inside
	// an internal error or another branch of errors.Join.
	var classified interface {
		error
		ErrorCode() apperror.Code
	}
	if errors.As(err, &classified) {
		if validation, ok := classified.(*apperror.ValidationError); ok && validation != nil {
			for _, detail := range validation.Details {
				details = append(details, FieldError{Field: detail.Field, Message: detail.Message})
			}
		}
	}
	write(w, r, status, code, message, details...)
}
func classify(err error) (int, string, string) {
	code := apperror.CodeOf(err)
	switch code {
	case apperror.ErrInvalidInput:
		return http.StatusBadRequest, string(code), "Invalid input"
	case apperror.ErrUnauthorized:
		return http.StatusUnauthorized, string(code), "Authentication required"
	case apperror.ErrForbidden:
		return http.StatusForbidden, string(code), "Access forbidden"
	case apperror.Code(errInvalidRequest):
		return 400, string(code), "Invalid JSON request"
	case apperror.Code(errBodyTooLarge):
		return 413, string(code), "Request body too large"
	case apperror.Code(errMediaType):
		return 415, string(code), "Content-Type must be application/json"
	case apperror.ErrAddressNotFound:
		return http.StatusNotFound, string(code), "Address not found"
	case apperror.ErrInvalidAddress:
		return http.StatusBadRequest, string(code), "Invalid address"
	case apperror.ErrInvalidCoordinates:
		return http.StatusBadRequest, string(code), "Invalid coordinates"
	case apperror.ErrProviderUnavailable:
		return http.StatusServiceUnavailable, string(code), "Provider unavailable"
	case apperror.ErrRateLimitExceeded:
		return http.StatusTooManyRequests, string(code), "Rate limit exceeded"
	default:
		return http.StatusInternalServerError, string(apperror.ErrInternal), "Internal server error"
	}
}
func write(w http.ResponseWriter, r *http.Request, status int, code, message string, details ...FieldError) {
	w.Header().Del("Content-Length")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	// A write failure cannot be replaced after headers have been committed.
	_ = json.NewEncoder(w).Encode(ErrorResponse{Success: false, Status: status, Error: ErrorDetail{
		Details: details,
		Code:    code, Message: message, RequestID: logging.RequestID(r.Context()),
	}})
}

func NotFound(w http.ResponseWriter, r *http.Request) {
	write(w, r, http.StatusNotFound, "ROUTE_NOT_FOUND", "Route not found")
}

// MethodNotAllowed requires the route registration to set the Allow header.
func MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	write(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
}
