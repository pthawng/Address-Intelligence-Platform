package httpserver

import (
	"address-intelligence-platform/internal/platform/apperror"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Limiter bounds aggregate requests per process, without an unbounded client map.
// Enforce distributed/user quotas at an ingress or shared limiter when needed.
type Limiter struct {
	mu                  sync.Mutex
	rate, burst, tokens float64
	last                time.Time
}

func NewLimiter(rate, burst int) (*Limiter, error) {
	if rate <= 0 || burst <= 0 {
		return nil, fmt.Errorf("rate and burst must be positive")
	}
	return &Limiter{rate: float64(rate), burst: float64(burst), tokens: float64(burst), last: time.Now()}, nil
}
func (l *Limiter) allow(now time.Time) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tokens = math.Min(l.burst, l.tokens+math.Max(0, now.Sub(l.last).Seconds())*l.rate)
	l.last = now
	if l.tokens >= 1 {
		l.tokens--
		return true, 0
	}
	return false, int(math.Max(1, math.Ceil((1-l.tokens)/l.rate)))
}
func (l *Limiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ok, retry := l.allow(time.Now()); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			WriteError(w, r, apperror.ErrRateLimitExceeded)
			return
		}
		next.ServeHTTP(w, r)
	})
}
