package httpserver

import (
	"address-intelligence-platform/internal/platform/apperror"
	"context"
	"net/http"
	"strings"
)

type Principal struct{ Subject string }
type principalKey struct{}

func CurrentPrincipal(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// Authenticator validates credentials (signature, issuer, audience, expiry and
// revocation as appropriate). Transport never trusts an unsigned decoded token.
type Authenticator interface {
	Authenticate(context.Context, string) (Principal, error)
}
type Authorizer interface {
	Authorize(context.Context, Principal, string) error
}

// Protected requires both authentication and an explicit permission policy.
// Missing wiring fails closed; no public fallback is installed.
func Protected(auth Authenticator, policy Authorizer, permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth == nil || policy == nil || permission == "" {
			WriteError(w, r, apperror.ErrInternal)
			return
		}
		values := r.Header.Values("Authorization")
		fields := strings.Fields(r.Header.Get("Authorization"))
		if len(values) != 1 || len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
			w.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
			WriteError(w, r, apperror.ErrUnauthorized)
			return
		}
		principal, err := auth.Authenticate(r.Context(), fields[1])
		if err != nil {
			if apperror.CodeOf(err) == apperror.ErrUnauthorized {
				w.Header().Set("WWW-Authenticate", `Bearer realm="api", error="invalid_token"`)
				WriteError(w, r, apperror.ErrUnauthorized)
			} else {
				WriteError(w, r, apperror.ErrInternal)
			}
			return
		}
		if principal.Subject == "" {
			WriteError(w, r, apperror.ErrInternal)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), principalKey{}, principal))
		if err := policy.Authorize(r.Context(), principal, permission); err != nil {
			if apperror.CodeOf(err) == apperror.ErrForbidden {
				WriteError(w, r, apperror.ErrForbidden)
			} else {
				WriteError(w, r, apperror.ErrInternal)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (router *Router) HandleProtected(pattern, permission string, auth Authenticator, policy Authorizer, handler Handler) {
	router.HandleHTTP(pattern, Protected(auth, policy, permission, Adapt(handler)))
}
