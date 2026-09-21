package bootstrap

import "net/http"

func healthRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", health)
	// Skeleton readiness only; add dependency probes when adapters are wired.
	mux.HandleFunc("GET /health/ready", health)
	return mux
}

func health(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte("{\"status\":\"ok\"}\n"))
}
