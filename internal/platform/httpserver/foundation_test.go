package httpserver

import (
	"address-intelligence-platform/internal/platform/apperror"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
func TestServerConfiguration(t *testing.T) {
	cfg := Config{ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: time.Minute, ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 32768}
	server, err := New(context.Background(), cfg, NewRouter(), quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	if server.ReadTimeout != cfg.ReadTimeout || server.WriteTimeout != cfg.WriteTimeout || server.IdleTimeout != cfg.IdleTimeout || server.ReadHeaderTimeout != cfg.ReadHeaderTimeout || server.MaxHeaderBytes != cfg.MaxHeaderBytes {
		t.Fatal("limits not wired")
	}
	for _, change := range []func(*Config){
		func(c *Config) { c.ReadTimeout = 0 }, func(c *Config) { c.WriteTimeout = -1 },
		func(c *Config) { c.IdleTimeout = 0 }, func(c *Config) { c.ReadHeaderTimeout = 0 }, func(c *Config) { c.MaxHeaderBytes = 0 },
	} {
		invalid := cfg
		change(&invalid)
		if _, err := New(context.Background(), invalid, NewRouter(), quietLogger()); err == nil {
			t.Fatal("unsafe configuration accepted")
		}
	}
	if _, err := New(context.Background(), cfg, nil, quietLogger()); err == nil {
		t.Fatal("nil handler accepted")
	}
}
func TestRouter(t *testing.T) {
	router := NewRouter()
	router.Handle("GET /places/{id}", func(w http.ResponseWriter, r *http.Request) error {
		return JSON(w, r, 200, map[string]string{"id": r.PathValue("id")})
	})
	router.Handle("POST /places/{id}", func(w http.ResponseWriter, r *http.Request) error { return apperror.ErrInvalidAddress })
	router.Handle("GET /tree/", func(w http.ResponseWriter, r *http.Request) error { return JSON(w, r, 200, true) })
	for _, tc := range []struct {
		method, path    string
		status          int
		contains, allow string
	}{
		{"GET", "/places/42", 200, `"id":"42"`, ""},
		{"POST", "/places/42", 400, "INVALID_ADDRESS", ""},
		{"DELETE", "/places/42", 405, "METHOD_NOT_ALLOWED", "GET, HEAD, POST"},
		{"GET", "/missing", 404, "ROUTE_NOT_FOUND", ""},
		{"GET", "/tree", 301, "", ""},
		{"HEAD", "/places/42", 200, "", ""},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.contains) || w.Header().Get("Allow") != tc.allow {
			t.Fatalf("%s %s: %d %s %v", tc.method, tc.path, w.Code, w.Body.String(), w.Header())
		}
		if tc.method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD has body")
		}
	}
}
func TestDecodeJSON(t *testing.T) {
	for _, tc := range []struct {
		name, body, content string
		limit               int64
		status              int
	}{
		{"valid", `{"name":"a"}`, "application/json; charset=utf-8", 100, 200},
		{"unknown", `{"other":1}`, "application/json", 100, 400},
		{"type", `{"name":1}`, "application/json", 100, 400},
		{"empty", "", "application/json", 100, 400},
		{"null", "null", "application/json", 100, 400},
		{"syntax", "{", "application/json", 100, 400},
		{"multiple", `{} {}`, "application/json", 100, 400},
		{"trailing", `{} garbage`, "application/json", 100, 400},
		{"media", `{}`, "text/plain", 100, 415},
		{"missing media", `{}`, "", 100, 415},
		{"large", `{"name":"abcdef"}`, "application/json", 5, 413},
		{"large whitespace", `{}                 `, "application/json", 5, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, chunked := range []bool{false, true} {
				r := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
				r.Header.Set("Content-Type", tc.content)
				if chunked {
					r.ContentLength = -1
				}
				w := httptest.NewRecorder()
				Adapt(func(w http.ResponseWriter, r *http.Request) error {
					var dst struct {
						Name string `json:"name"`
					}
					if err := DecodeJSON(w, r, &dst, tc.limit); err != nil {
						return err
					}
					return JSON(w, r, 200, dst)
				}).ServeHTTP(w, r)
				if w.Code != tc.status {
					t.Fatalf("chunked=%v: %d %s", chunked, w.Code, w.Body.String())
				}
			}
		})
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	WriteError(w, r, DecodeJSON(w, r, nil, 100))
	if w.Code != 500 {
		t.Fatal("programming error became client error")
	}
	status, _, _ := classify(&apperror.AppError{Code: apperror.ErrInternal, Err: errInvalidRequest})
	if status != 500 {
		t.Fatal("outer classification lost")
	}
}
func TestRecoveryAndEncoding(t *testing.T) {
	for _, mode := range []string{"panic", "encoding"} {
		w := httptest.NewRecorder()
		h := Adapt(func(w http.ResponseWriter, r *http.Request) error {
			if mode == "panic" {
				w.Header().Set("Content-Length", "900")
				w.Header().Set("Set-Cookie", "secret")
				panic("private detail")
			}
			return JSON(w, r, 200, make(chan int))
		})
		Middleware(quietLogger(), h).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		var body ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != 500 || body.Error.Code != "INTERNAL_ERROR" || body.Error.RequestID == "" || w.Header().Get("Set-Cookie") != "" || w.Header().Get("Content-Length") != "" {
			t.Fatalf("unsafe recovery: %v %s", w.Header(), w.Body.String())
		}
	}
}
func TestRecoveryAfterCommitAborts(t *testing.T) {
	for _, mode := range []string{"write", "flush", "abort"} {
		t.Run(mode, func(t *testing.T) {
			w := httptest.NewRecorder()
			defer func() {
				if recover() != http.ErrAbortHandler {
					t.Error("expected abort")
				}
				if strings.Contains(w.Body.String(), "INTERNAL_ERROR") {
					t.Error("appended error")
				}
			}()
			Middleware(quietLogger(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if mode == "abort" {
					panic(http.ErrAbortHandler)
				}
				if mode == "flush" {
					_ = http.NewResponseController(w).Flush()
				} else {
					_, _ = w.Write([]byte("partial"))
				}
				panic("private detail")
			})).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
		})
	}
}

func TestServerRejectsOversizedHeaders(t *testing.T) {
	configured, err := New(context.Background(), Config{ReadTimeout: time.Second, WriteTimeout: time.Second, IdleTimeout: time.Second, ReadHeaderTimeout: time.Second, MaxHeaderBytes: 1024}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("oversized request reached handler") }), quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(configured.Handler)
	server.Config = configured
	server.Start()
	defer server.Close()
	request, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Large", strings.Repeat("x", 16384))
	client := server.Client()
	client.Timeout = 3 * time.Second
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 431 {
		t.Fatalf("status=%d", response.StatusCode)
	}
}
