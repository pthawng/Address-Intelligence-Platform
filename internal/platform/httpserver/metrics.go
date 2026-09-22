package httpserver

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Metrics uses bounded status-class counters; no paths, IDs or user labels.
type Metrics struct {
	requests   [6]atomic.Uint64
	durationNS atomic.Uint64
	inflight   atomic.Int64
}

func (m *Metrics) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m.inflight.Add(1)
		response := &metricWriter{ResponseWriter: w}
		defer func() {
			m.inflight.Add(-1)
			status := response.status
			if status == 0 {
				status = 500
			}
			class := status / 100
			if class < 1 || class > 5 {
				class = 0
			}
			m.requests[class].Add(1)
			m.durationNS.Add(uint64(time.Since(start)))
		}()
		next.ServeHTTP(response, r)
		if response.status == 0 {
			response.status = 200
		}
	})
}

type metricWriter struct {
	http.ResponseWriter
	status int
}

func (w *metricWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *metricWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.ResponseWriter.WriteHeader(status)
	if status >= 200 || status == 101 {
		w.status = status
	}
}
func (w *metricWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(p)
}
func (w *metricWriter) FlushError() error {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return http.NewResponseController(w.ResponseWriter).Flush()
}

// ServeHTTP exposes Prometheus text. Mount only on a private/authenticated route.
func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(200)
	if r.Method == "HEAD" {
		return
	}
	fmt.Fprintln(w, "# TYPE http_requests_total counter")
	var count uint64
	for class := range m.requests {
		v := m.requests[class].Load()
		count += v
		fmt.Fprintf(w, `http_requests_total{status_class="%dxx"} %d`+"\n", class, v)
	}
	fmt.Fprintln(w, "# TYPE http_request_duration_seconds summary")
	fmt.Fprintf(w, "http_request_duration_seconds_sum %g\nhttp_request_duration_seconds_count %d\n", float64(m.durationNS.Load())/1e9, count)
	fmt.Fprintf(w, "# TYPE http_requests_inflight gauge\nhttp_requests_inflight %d\n", m.inflight.Load())
}
