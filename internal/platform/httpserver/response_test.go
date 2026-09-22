package httpserver

import (
	"address-intelligence-platform/internal/platform/apperror"
	"encoding/json"
	"fmt"
	"math"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSuccessEnvelope(t *testing.T) {
	for _, status := range []int{200, 201} {
		w := httptest.NewRecorder()
		err := Success(w, httptest.NewRequest("GET", "/", nil), status, "Done", map[string]any{"id": 1})
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Success bool
			Status  int
			Message string
			Data    map[string]int
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if !body.Success || body.Status != status || w.Code != status || body.Message != "Done" || body.Data["id"] != 1 {
			t.Fatal(w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	if err := JSON(w, httptest.NewRequest("GET", "/", nil), 200, nil); err != nil {
		t.Fatal(err)
	}
	if w.Body.String() != `{"success":true,"status":200,"data":null}`+"\n" {
		t.Fatal(w.Body.String())
	}
	for _, status := range []int{199, 204, 205, 301, 400, 500} {
		w := httptest.NewRecorder()
		if err := JSON(w, httptest.NewRequest("GET", "/", nil), status, true); err == nil {
			t.Fatal("invalid status accepted", status)
		}
		if w.Body.Len() != 0 || len(w.Header()) != 0 {
			t.Fatal("committed invalid response")
		}
	}
}
func TestNoContentAndHead(t *testing.T) {
	w := httptest.NewRecorder()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", "100")
	if err := NoContent(w, httptest.NewRequest("DELETE", "/", nil)); err != nil {
		t.Fatal(err)
	}
	if w.Code != 204 || w.Body.Len() != 0 || w.Header().Get("Content-Type") != "" || w.Header().Get("Content-Length") != "" {
		t.Fatal("invalid 204")
	}
	w = httptest.NewRecorder()
	if err := JSON(w, httptest.NewRequest("HEAD", "/", nil), 200, true); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Length") == "" {
		t.Fatal("invalid HEAD")
	}
}
func TestPaginationEnvelope(t *testing.T) {
	for _, tc := range []struct {
		page, limit, total, pages int64
		items                     []int
	}{
		{1, 10, 50, 5, []int{1, 2}}, {2, 10, 11, 2, []int{11}}, {1, 10, 0, 0, nil},
		{7, 10, 50, 5, nil}, {1, 2, math.MaxInt64, math.MaxInt64/2 + 1, nil},
	} {
		w := httptest.NewRecorder()
		if err := Paginated(w, httptest.NewRequest("GET", "/", nil), tc.items, tc.page, tc.limit, tc.total); err != nil {
			t.Fatal(err)
		}
		var body struct {
			Success bool
			Status  int
			Data    []int
			Meta    PaginationMeta
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if !body.Success || body.Status != 200 || body.Meta.TotalPages != tc.pages || body.Meta.TotalRecords != tc.total || body.Meta.Page != tc.page || body.Meta.Limit != tc.limit || body.Data == nil {
			t.Fatal(w.Body.String())
		}
	}
	for _, tc := range []struct {
		page, limit, total int64
		items              []int
	}{
		{0, 10, 0, nil}, {1, 0, 0, nil}, {1, 10, -1, nil}, {2, 10, 1, []int{1}}, {1, 1, 2, []int{1, 2}}, {2, 10, 11, []int{1, 2}},
	} {
		w := httptest.NewRecorder()
		if err := Paginated(w, httptest.NewRequest("GET", "/", nil), tc.items, tc.page, tc.limit, tc.total); err == nil {
			t.Fatal("invalid pagination accepted")
		}
		if w.Body.Len() != 0 {
			t.Fatal("invalid metadata committed")
		}
	}
}
func TestValidationDetailsAndAuthentication(t *testing.T) {
	validation := &apperror.ValidationError{Details: []apperror.FieldViolation{{Field: "email", Message: "Invalid email format"}}}
	for _, tc := range []struct {
		err     error
		status  int
		details bool
	}{
		{fmt.Errorf("private context: %w", validation), 400, true},
		{&apperror.AppError{Code: apperror.ErrInternal, Err: validation}, 500, false},
		{&apperror.AppError{Code: apperror.ErrInvalidInput, Err: validation}, 400, false},
		{apperror.ErrUnauthorized, 401, false}, {apperror.ErrForbidden, 403, false},
	} {
		w := httptest.NewRecorder()
		WriteError(w, httptest.NewRequest("GET", "/", nil), tc.err)
		var body ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Success || body.Status != tc.status || w.Code != tc.status || (len(body.Error.Details) > 0) != tc.details || strings.Contains(w.Body.String(), "private") {
			t.Fatal(w.Body.String())
		}
		if tc.details && body.Error.Details[0].Field != "email" {
			t.Fatal("field missing")
		}
	}
}
func TestTimestampUTC(t *testing.T) {
	value := time.Date(2026, 9, 23, 4, 58, 0, 0, time.FixedZone("ICT", 7*3600))
	w := httptest.NewRecorder()
	if err := JSON(w, httptest.NewRequest("GET", "/", nil), 200, struct {
		CreatedAt time.Time `json:"created_at"`
	}{Timestamp(value)}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(w.Body.String(), `"created_at":"2026-09-22T21:58:00Z"`) {
		t.Fatal(w.Body.String())
	}
}
