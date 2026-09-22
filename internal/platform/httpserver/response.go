package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// SuccessResponse is the common envelope for successful JSON responses.
type SuccessResponse struct {
	Success bool            `json:"success"`
	Status  int             `json:"status"`
	Message string          `json:"message,omitempty"`
	Data    any             `json:"data"`
	Meta    *PaginationMeta `json:"meta,omitempty"`
}

// JSON writes the standard success envelope. Use WriteError for failures.
func JSON(w http.ResponseWriter, r *http.Request, status int, value any) error {
	return Success(w, r, status, "", value)
}

func Success(w http.ResponseWriter, r *http.Request, status int, message string, data any) error {
	if status < 200 || status >= 300 {
		return fmt.Errorf("success requires a 2xx status")
	}
	return writeJSON(w, r, status, SuccessResponse{Success: true, Status: status, Message: message, Data: data})
}

// NoContent deliberately has no JSON envelope, as required by HTTP 204.
func NoContent(w http.ResponseWriter, _ *http.Request) error {
	w.Header().Del("Content-Type")
	w.Header().Del("Content-Length")
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// writeJSON encodes before commitment; write failures abort instead of replying twice.
func writeJSON(w http.ResponseWriter, r *http.Request, status int, value any) error {
	if status < 200 || status > 599 || status == http.StatusNoContent || status == http.StatusResetContent || status == http.StatusNotModified {
		return fmt.Errorf("JSON requires a final status that permits a body")
	}
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode JSON response: %w", err)
	}
	body = append(body, '\n')
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return nil
	}
	if _, err := w.Write(body); err != nil {
		panic(http.ErrAbortHandler)
	}
	return nil
}
