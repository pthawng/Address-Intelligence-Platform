package httpserver

import (
	"address-intelligence-platform/internal/platform/apperror"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

const DefaultMaxBodyBytes int64 = 1 << 20 // 1 MiB for JSON endpoints.
type requestError string

func (e requestError) Error() string            { return string(e) }
func (e requestError) ErrorCode() apperror.Code { return apperror.Code(e) }

const (
	errInvalidRequest requestError = "INVALID_REQUEST"
	errBodyTooLarge   requestError = "REQUEST_TOO_LARGE"
	errMediaType      requestError = "UNSUPPORTED_MEDIA_TYPE"
)

// DecodeJSON accepts one non-null JSON value, rejects unknown struct fields,
// and limits reads including chunked bodies. dst must be a non-nil pointer.
// The handler must return its error before writing a response.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any, maxBytes int64) error {
	if maxBytes <= 0 {
		return errors.New("JSON body limit must be positive")
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errMediaType
	}
	if r.ContentLength > maxBytes {
		return errBodyTooLarge
	}
	if r.Body == nil {
		return errInvalidRequest
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	decoder := json.NewDecoder(r.Body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return decodeError(err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return decodeError(err)
		}
		return errInvalidRequest
	}
	if string(raw) == "null" {
		return errInvalidRequest
	}
	// Decode into the destination only after the size/single-value checks pass.
	return decodeValue(raw, dst)
}
func decodeError(err error) error {
	var large *http.MaxBytesError
	if errors.As(err, &large) {
		return errBodyTooLarge
	}
	var invalid *json.InvalidUnmarshalError
	if errors.As(err, &invalid) {
		return err
	}
	return errInvalidRequest
}

func decodeValue(raw []byte, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return decodeError(err)
	}
	return nil
}
