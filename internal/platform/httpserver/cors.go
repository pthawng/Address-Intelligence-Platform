package httpserver

import (
	"address-intelligence-platform/internal/platform/apperror"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// CORS uses explicit origins; credentials and wildcard origins are not enabled.
func CORS(origins []string, next http.Handler) (http.Handler, error) {
	allowed := map[string]bool{}
	for _, origin := range origins {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || strings.Contains(origin, "*") {
			return nil, fmt.Errorf("CORS origins must be explicit http(s) origins without paths")
		}
		allowed[origin] = true
	}
	methods := map[string]bool{"GET": true, "HEAD": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "OPTIONS": true}
	headers := map[string]bool{"authorization": true, "content-type": true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		if !allowed[origin] {
			WriteError(w, r, apperror.ErrForbidden)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, Retry-After, Location")
		if r.Method == "OPTIONS" && r.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			if !methods[r.Header.Get("Access-Control-Request-Method")] {
				WriteError(w, r, apperror.ErrForbidden)
				return
			}
			for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
				header = strings.ToLower(strings.TrimSpace(header))
				if header != "" && !headers[header] {
					WriteError(w, r, apperror.ErrForbidden)
					return
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	}), nil
}
