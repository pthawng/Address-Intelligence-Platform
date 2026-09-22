package httpserver

import (
	"address-intelligence-platform/internal/platform/apperror"
	"address-intelligence-platform/internal/platform/logging"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRecoveryLogsStackWithoutPayload(t *testing.T) {
	var output bytes.Buffer
	logger := logging.New(logging.Options{Output: &output})
	w := httptest.NewRecorder()
	Middleware(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("private-token") })).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 500 || !strings.Contains(output.String(), "stack_trace") || !strings.Contains(output.String(), "TestRecoveryLogsStackWithoutPayload") || strings.Contains(output.String(), "private-token") {
		t.Fatal(output.String())
	}
	if strings.Count(output.String(), "HTTP handler panic") != 1 {
		t.Fatal("panic logged more than once")
	}
	if !strings.Contains(output.String(), w.Header().Get("X-Request-ID")) {
		t.Fatal("missing correlation")
	}
}
func TestCORS(t *testing.T) {
	for _, tc := range []struct {
		origin, method, requested, headers string
		status                             int
	}{
		{"http://localhost:3000", "GET", "", "", 200},
		{"http://localhost:3000", "OPTIONS", "POST", "authorization, content-type", 204},
		{"http://localhost:3000", "OPTIONS", "POST", "x-untrusted", 403},
		{"http://localhost:3000", "OPTIONS", "BOGUS", "", 403},
		{"https://evil.example", "GET", "", "", 403},
		{"", "GET", "", "", 200},
	} {
		h, err := CORS([]string{"http://localhost:3000"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(tc.method, "/", nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Access-Control-Request-Method", tc.requested)
		r.Header.Set("Access-Control-Request-Headers", tc.headers)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%+v: %d", tc, w.Code)
		}
		if tc.origin == "https://evil.example" && w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("untrusted origin allowed")
		}
		if tc.origin == "http://localhost:3000" && w.Header().Get("Access-Control-Allow-Origin") != tc.origin {
			t.Fatal("missing allow origin")
		}
	}
	if _, err := CORS([]string{"*"}, http.NotFoundHandler()); err == nil {
		t.Fatal("wildcard accepted")
	}
}

type authFunc func(context.Context, string) (Principal, error)

func (f authFunc) Authenticate(c context.Context, s string) (Principal, error) { return f(c, s) }

type policyFunc func(context.Context, Principal, string) error

func (f policyFunc) Authorize(c context.Context, p Principal, s string) error { return f(c, p, s) }
func TestProtectedRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, header       string
		authErr, policyErr error
		status             int
	}{
		{"missing", "", nil, nil, 401}, {"malformed", "Basic abc", nil, nil, 401},
		{"invalid", "Bearer bad", apperror.ErrUnauthorized, nil, 401},
		{"unavailable", "Bearer token", errors.New("provider detail"), nil, 500},
		{"denied", "Bearer token", nil, apperror.ErrForbidden, 403},
		{"policy failure", "Bearer token", nil, errors.New("private policy"), 500},
		{"allowed", "Bearer token", nil, nil, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			policyCalled := false
			h := Protected(authFunc(func(ctx context.Context, token string) (Principal, error) {
				return Principal{Subject: "user"}, tc.authErr
			}), policyFunc(func(ctx context.Context, p Principal, permission string) error {
				policyCalled = true
				if permission != "places:read" || p.Subject != "user" {
					t.Error("invalid authorization context")
				}
				return tc.policyErr
			}), "places:read", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if p, ok := CurrentPrincipal(r.Context()); !ok || p.Subject != "user" {
					t.Error("principal not propagated")
				}
				w.WriteHeader(200)
			}))
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("Authorization", tc.header)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || called != (tc.status == 200) {
				t.Fatal(w.Code, w.Body.String())
			}
			if tc.status == 401 && w.Header().Get("WWW-Authenticate") == "" {
				t.Fatal("missing challenge")
			}
			if (tc.header == "" || tc.authErr != nil) && policyCalled {
				t.Fatal("authorization before authentication")
			}
		})
	}
	w := httptest.NewRecorder()
	Protected(nil, nil, "", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("fail open") })).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 500 {
		t.Fatal("missing wiring accepted")
	}
}
func TestLimiterConcurrentAndRefill(t *testing.T) {
	limiter, _ := NewLimiter(10, 5)
	start := limiter.last
	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted := 0
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := limiter.allow(start); ok {
				mu.Lock()
				accepted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if accepted != 5 {
		t.Fatal(accepted)
	}
	if ok, _ := limiter.allow(start.Add(time.Second)); !ok {
		t.Fatal("no refill")
	}
	limiter, _ = NewLimiter(1, 1)
	h := limiter.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatal(w.Code, w.Header())
	}
}
func TestTimeoutAndLateWrite(t *testing.T) {
	release, done := make(chan struct{}), make(chan error, 1)
	h := Timeout(10*time.Millisecond, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		<-release
		_, err := w.Write([]byte("late"))
		done <- err
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r = r.WithContext(logging.WithRequestID(r.Context(), "test-id"))
	h.ServeHTTP(w, r)
	close(release)
	if err := <-done; !errors.Is(err, http.ErrHandlerTimeout) {
		t.Fatal(err)
	}
	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(w.Body.String(), err)
	}
	if w.Code != 503 || body.Status != 503 || body.Error.Code != "REQUEST_TIMEOUT" || body.Error.RequestID != "test-id" || w.Header().Get("Content-Type") != "application/json" {
		t.Fatal(w.Code, w.Header(), w.Body.String())
	}
}
func TestStackMetricsAndPreflight(t *testing.T) {
	metrics := new(Metrics)
	h, err := stack(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }), MiddlewareOptions{Origins: []string{"http://localhost:3000"}, Timeout: time.Second, RatePerSecond: 1, Burst: 1, Metrics: metrics})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/data", nil))
		if i == 1 && w.Code != 429 {
			t.Fatal(w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/health/live", nil))
	if w.Code != 200 {
		t.Fatal("rate-limited probe")
	}
	r := httptest.NewRequest("OPTIONS", "/data", nil)
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Access-Control-Request-Method", "GET")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatal("preflight hit limit")
	}
	w = httptest.NewRecorder()
	metrics.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(w.Body.String(), `http_requests_total{status_class="4xx"} 1`) || metrics.inflight.Load() != 0 {
		t.Fatal(w.Body.String())
	}
}
