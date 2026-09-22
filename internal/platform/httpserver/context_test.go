package httpserver

import (
	"address-intelligence-platform/internal/platform/logging"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Exercise the real middleware over a real connection: cancellation must reach
// downstream work without dropping request identity or the tighter parent limit.
func TestClientCancellationReachesHandler(t *testing.T) {
	entered := make(chan bool, 1)
	finished := make(chan error, 1)
	handler, err := stack(quietLogger(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Hour)
		defer cancel()
		deadline, ok := ctx.Deadline()
		entered <- ok && time.Until(deadline) <= 5*time.Second && logging.RequestID(ctx) != ""
		<-ctx.Done()
		finished <- ctx.Err()
	}), MiddlewareOptions{Timeout: 5 * time.Second, RatePerSecond: 100, Burst: 100})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	clientDone := make(chan error, 1)
	go func() {
		response, err := server.Client().Do(request)
		if response != nil {
			response.Body.Close()
		}
		clientDone <- err
	}()
	select {
	case valid := <-entered:
		if !valid {
			t.Error("deadline or request identity was lost")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not start")
	}
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("downstream cancellation: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("client cancellation did not reach downstream work")
	}
	select {
	case err := <-clientDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("client cancellation: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("client did not stop")
	}
}
