package bootstrap

import (
	"address-intelligence-platform/internal/platform/httpserver"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDatabaseReadinessProbe(t *testing.T) {
	for _, available := range []bool{false, true} {
		probe := func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Error("probe has no deadline")
			}
			if !available {
				return errors.New("private database error")
			}
			return nil
		}
		router := databaseHealthRoutes(probe, time.Second)
		for _, path := range []string{"/health/live", "/health/ready"} {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
			want := 200
			if path == "/health/ready" && !available {
				want = 503
			}
			if w.Code != want {
				t.Fatalf("%s status=%d want=%d", path, w.Code, want)
			}
		}
	}
}

func TestHealthRoutingErrors(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		status       int
		code, allow  string
	}{
		{"GET", "/missing", 404, "ROUTE_NOT_FOUND", ""},
		{"POST", "/health/live", 405, "METHOD_NOT_ALLOWED", "GET, HEAD"},
		{"OPTIONS", "/health/ready", 405, "METHOD_NOT_ALLOWED", "GET, HEAD"},
		{"GET", "/health/live", 200, "", ""},
		{"GET", "/health/ready", 503, "PROVIDER_UNAVAILABLE", ""},
		{"HEAD", "/missing", 404, "", ""},
	} {
		w := httptest.NewRecorder()
		healthRoutes().ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status || w.Header().Get("Allow") != tc.allow {
			t.Fatalf("%s %s: %d %v", tc.method, tc.path, w.Code, w.Header())
		}
		if tc.code != "" {
			var body httpserver.ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error.Code != tc.code {
				t.Fatal(body)
			}
		}
		if tc.method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD wrote body")
		}
	}
}

func TestHealthSuccessEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	healthRoutes().ServeHTTP(w, httptest.NewRequest("GET", "/health/live", nil))
	var body struct {
		Success bool
		Status  int
		Data    struct{ Status string }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Success || body.Status != 200 || body.Data.Status != "ok" {
		t.Fatal(w.Body.String())
	}
}
