package bootstrap

import (
	"address-intelligence-platform/internal/platform/apperror"
	"address-intelligence-platform/internal/platform/httpserver"
	"context"
	"net/http"
	"time"
)

func healthRoutes(metrics ...*httpserver.Metrics) http.Handler {
	return databaseHealthRoutes(nil, time.Second, metrics...)
}

func databaseHealthRoutes(probe func(context.Context) error, timeout time.Duration, metrics ...*httpserver.Metrics) http.Handler {
	router := httpserver.NewRouter()
	if len(metrics) > 0 {
		router.HandleHTTP("GET /metrics", metrics[0])
	}
	router.Handle("GET /health/live", health)
	router.Handle("GET /health/ready", func(w http.ResponseWriter, r *http.Request) error {
		if probe == nil {
			return apperror.ErrProviderUnavailable
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		if err := probe(ctx); err != nil {
			return apperror.ErrProviderUnavailable
		}
		return health(w, r)
	})
	return router
}
func health(w http.ResponseWriter, r *http.Request) error {
	return httpserver.JSON(w, r, http.StatusOK, struct {
		Status string `json:"status"`
	}{Status: "ok"})
}
