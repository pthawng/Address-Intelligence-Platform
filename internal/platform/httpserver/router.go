package httpserver

import "net/http"

// Router uses Go ServeMux patterns (including methods and path wildcards).
// Register routes during startup, before serving. Duplicate/conflicting patterns
// panic, matching ServeMux's startup validation.
type Router struct{ mux *http.ServeMux }

func NewRouter() *Router                                          { return &Router{mux: http.NewServeMux()} }
func (r *Router) Handle(pattern string, handler Handler)          { r.mux.Handle(pattern, Adapt(handler)) }
func (r *Router) HandleHTTP(pattern string, handler http.Handler) { r.mux.Handle(pattern, handler) }
func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h, pattern := router.mux.Handler(r)
	if pattern != "" {
		// ServeHTTP populates PathValue; calling Handler's result directly does not.
		router.mux.ServeHTTP(w, r)
		return
	}
	// Only intercept generated routing failures, never an endpoint's response.
	// Preserve ServeMux's Allow calculation and canonical-path redirects.
	h.ServeHTTP(&routingResponse{ResponseWriter: w, request: r}, r)
}

type routingResponse struct {
	http.ResponseWriter
	request *http.Request
	mapped  bool
}

func (w *routingResponse) WriteHeader(status int) {
	switch status {
	case http.StatusNotFound:
		w.mapped = true
		NotFound(w.ResponseWriter, w.request)
	case http.StatusMethodNotAllowed:
		w.mapped = true
		MethodNotAllowed(w.ResponseWriter, w.request)
	default:
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *routingResponse) Write(data []byte) (int, error) {
	if w.mapped {
		return len(data), nil
	}
	return w.ResponseWriter.Write(data)
}
