package httpserver

import (
	"address-intelligence-platform/internal/platform/apperror"
	"address-intelligence-platform/internal/platform/logging"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestErrorContract(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{apperror.ErrAddressNotFound, 404, "ADDRESS_NOT_FOUND"},
		{apperror.ErrInvalidAddress, 400, "INVALID_ADDRESS"},
		{apperror.ErrInvalidCoordinates, 400, "INVALID_COORDINATES"},
		{apperror.ErrProviderUnavailable, 503, "PROVIDER_UNAVAILABLE"},
		{apperror.ErrRateLimitExceeded, 429, "RATE_LIMIT_EXCEEDED"},
		{apperror.ErrInternal, 500, "INTERNAL_ERROR"},
		{errors.New("secret password"), 500, "INTERNAL_ERROR"},
		{&apperror.AppError{Code: "UNKNOWN", Err: apperror.ErrInvalidAddress}, 500, "INTERNAL_ERROR"},
		{&apperror.AppError{Code: apperror.ErrInternal, Err: apperror.ErrAddressNotFound}, 500, "INTERNAL_ERROR"},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/test", nil)
			r.Header.Set("X-Request-ID", "untrusted")
			h := logging.HTTP(slog.New(slog.NewTextHandler(io.Discard, nil)), Adapt(func(http.ResponseWriter, *http.Request) error {
				return fmt.Errorf("secret password: %w", tc.err)
			}))
			h.ServeHTTP(w, r)
			var body ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || body.Status != tc.status || body.Success || body.Error.Code != tc.code {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if body.Error.Message == "" || strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "UNKNOWN") {
				t.Fatal("unsafe message")
			}
			if body.Error.RequestID == "" || body.Error.RequestID == "untrusted" || body.Error.RequestID != w.Header().Get("X-Request-ID") {
				t.Fatal("invalid correlation")
			}
			if w.Header().Get("Content-Type") != "application/json" || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing headers")
			}
		})
	}
}
func TestNilAndHEAD(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("HEAD", "/test", nil)
	WriteError(w, r, nil)
	if len(w.Header()) != 0 || w.Body.Len() != 0 {
		t.Fatal("nil changed response")
	}
	WriteError(w, r, apperror.ErrInternal)
	if w.Code != 500 || w.Body.Len() != 0 {
		t.Fatal("HEAD body must be empty")
	}
}
func TestAdaptSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	Adapt(func(w http.ResponseWriter, r *http.Request) error { w.WriteHeader(204); return nil }).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatal("success changed")
	}
}
