package bootstrap

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	runtimeEnvironment(t)
	app, err := NewAPI()
	if err != nil {
		t.Fatal(err)
	}
	app.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return app
}

func TestStartupFailureRollsBackInReverseOrder(t *testing.T) {
	app := testApp(t)
	startupErr := errors.New("service initialization failed")
	closeErr := errors.New("close failed")
	var order []string
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.run = func(_ context.Context, owned *resources) error {
		owned.add("database", func(ctx context.Context) error {
			order = append(order, "database")
			if ctx.Err() != nil {
				t.Error("cleanup inherited canceled runtime context")
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Error("cleanup has no deadline")
			}
			return nil
		})
		owned.add("search", func(context.Context) error { order = append(order, "search"); return closeErr })
		cancel()
		return startupErr
	}
	err := app.Run(ctx)
	if !errors.Is(err, startupErr) || !errors.Is(err, closeErr) {
		t.Fatalf("lost startup or cleanup error: %v", err)
	}
	if !reflect.DeepEqual(order, []string{"search", "database"}) {
		t.Fatalf("cleanup order: %v", order)
	}
	if err := app.Run(context.Background()); err == nil {
		t.Fatal("second Run accepted")
	}
}

func TestResourcesCloseOnceAndContinueAfterDeadline(t *testing.T) {
	var owned resources
	var calls int
	for range 2 {
		owned.add("resource", func(ctx context.Context) error { calls++; return ctx.Err() })
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(owned.close(ctx), context.Canceled) {
		t.Fatal("cleanup errors lost")
	}
	if err := owned.close(ctx); err != nil || calls != 2 {
		t.Fatalf("cleanup repeated or skipped: calls=%d err=%v", calls, err)
	}
}

func TestCanceledRunAcquiresNothing(t *testing.T) {
	app := testApp(t)
	app.run = func(context.Context, *resources) error { t.Fatal("canceled Run initialized resources"); return nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := app.Run(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestListenerFailureIsReturned(t *testing.T) {
	app := testApp(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	app.cfg.HTTPAddress = listener.Addr().String()
	if err := app.Run(context.Background()); err == nil {
		t.Fatal("occupied address accepted")
	}
}

type observedListener struct {
	net.Listener
	closed chan struct{}
	once   sync.Once
}

func (l *observedListener) Close() error {
	err := l.Listener.Close()
	l.once.Do(func() { close(l.closed) })
	return err
}

func startHTTP(t *testing.T, app *App, handler http.Handler) (*observedListener, context.CancelFunc, <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	observed := &observedListener{Listener: listener, closed: make(chan struct{})}
	t.Cleanup(func() { _ = observed.Close() })
	app.run = func(ctx context.Context, owned *resources) error { return app.serveHTTP(ctx, owned, observed, handler) }
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	return observed, cancel, done
}

func await[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for lifecycle event")
		var zero T
		return zero
	}
}

func request(t *testing.T, listener net.Listener, path string) <-chan error {
	t.Helper()
	result := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		response, err := client.Get("http://" + listener.Addr().String() + path)
		if err == nil {
			_, err = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode != http.StatusOK {
				err = errors.New("unexpected health status")
			}
		}
		result <- err
	}()
	return result
}

func TestHTTPGracefulDrain(t *testing.T) {
	app := testApp(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
	listener, cancel, done := startHTTP(t, app, handler)
	response := request(t, listener, "/")
	await(t, entered)
	cancel()
	await(t, listener.closed)
	select {
	case err := <-done:
		t.Fatalf("Run returned before draining: %v", err)
	default:
	}
	unblock()
	if err := await(t, response); err != nil {
		t.Fatal(err)
	}
	if err := await(t, done); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPShutdownTimeoutCancelsRequests(t *testing.T) {
	app := testApp(t)
	app.cfg.ShutdownTimeout = 30 * time.Millisecond
	entered, canceled := make(chan struct{}), make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(canceled)
	})
	listener, cancel, done := startHTTP(t, app, handler)
	response := request(t, listener, "/")
	await(t, entered)
	cancel()
	if err := await(t, done); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected shutdown timeout, got %v", err)
	}
	await(t, canceled)
	await(t, response)
}

func TestUnexpectedServerFailureAndHealthRoutes(t *testing.T) {
	app := testApp(t)
	listener, _, done := startHTTP(t, app, healthRoutes())
	for _, path := range []string{"/health/live", "/health/ready"} {
		if err := await(t, request(t, listener, path)); err != nil {
			t.Fatal(err)
		}
	}
	_ = listener.Close()
	if err := await(t, done); err == nil {
		t.Fatal("unexpected listener failure was swallowed")
	}
}
