# Middleware Foundation

## Enabled API pipeline

Request ID -> tracing context -> access logging -> metrics -> recovery -> CORS ->
request timeout -> inner recovery -> rate limit -> router -> authentication ->
authorization -> handler.

Tracing initializes context before access logging so both completion and panic
logs carry request_id and trace_id. Incoming request IDs are replaced by a random
server ID. W3C traceparent is parsed by OpenTelemetry, not treated as authentication.
No global logger or tracer provider is modified.

## Recovery

A handler panic logs one ERROR diagnostic with a symbolized stack_trace, request
correlation and INTERNAL_ERROR. Function names, files and line numbers are logged;
panic payloads and function argument values are not. Uncommitted responses become
the standard JSON 500 envelope. Already committed responses abort with
http.ErrAbortHandler; sending a second 500 would corrupt the HTTP response.

Timeout executes the handler in a goroutine, so recovery is also installed inside
that goroutine. The outer recovery covers the surrounding middleware. Background
goroutines created by business code must have their own error/recovery boundary;
HTTP middleware cannot recover a panic in an unrelated goroutine.

## CORS

HTTP_CORS_ORIGINS is a comma-separated exact origin allowlist. Development/test
(default) origin: http://localhost:3000, as selected for this project. Production
and staging default to no allowed origins. Wildcards, credentials and URL paths
are not accepted. Allowed request headers are Authorization and Content-Type;
exposed headers are X-Request-ID, Retry-After and Location. Preflight runs before
rate limiting and route authentication. CORS is a browser policy, not authorization.
Cookie-based authentication is not enabled by this foundation.

## Authentication and authorization

```go
router.HandleProtected("GET /places/{id}", "places:read", authenticator, authorizer, handler.Get)
```

Protected routes require a Bearer token, an injected Authenticator and an explicit
Authorizer policy. Missing/invalid credentials produce 401 plus WWW-Authenticate;
denied permission produces 403. Missing wiring or provider/policy failures produce
a safe 500. A principal is put in context only after successful authentication.
The authorizer has access to the request context and verified subject; resource
ownership/tenant checks still belong in the application operation.

The selected strategy is JWT access tokens issued by an OIDC provider. Issuer and
API audience are not available yet, so no provider adapter or fake JWT acceptance
is installed. Implement Authenticator using the provider's verified JWKS and a
maintained verifier. Validate allowed algorithms, signature, issuer, API audience,
expiry/not-before and provider-specific token type; do not accept ID tokens as API
access tokens. Use discovery/key caching and bounded request timeouts. Return
ErrUnauthorized for invalid credentials; return other errors for verifier outages.
Never trust plain decoded claims or client-supplied roles. Authorizer must use
verified identity and server-side policy. Health routes are explicitly public;
new business routes should use HandleProtected. No business routes currently exist.

## Rate limit and timeout

HTTP_RATE_PER_SECOND=100 and HTTP_RATE_BURST=200 configure a synchronized token
bucket per API process, with bounded memory. Rejections return 429 and Retry-After.
GET/HEAD health probes and metrics are exempt. The limiter does not trust
X-Forwarded-For or create unbounded per-IP maps. Multiple instances need an ingress
or shared limiter for global/user quotas; this is an aggregate overload guard.

HTTP_REQUEST_TIMEOUT=8s must be shorter than HTTP_WRITE_TIMEOUT (default 15s).
The standard library timeout handler buffers responses and returns a JSON 503
REQUEST_TIMEOUT, cancels the request context and rejects late writes. This is for
bounded JSON endpoints, not streaming/hijacking or large downloads. Go cannot kill
a goroutine: handlers and provider/database operations must honor cancellation.
Do not create unbounded response bodies; use pagination. Client cancellation can
close the connection without a JSON body. WriteTimeout remains a socket deadline.

## Tracing and metrics

OpenTelemetry is isolated in platform/telemetry. It creates server spans and log
correlation. Sampling is fixed at 10%, independent of a caller's sampled flag to
avoid client-driven export amplification. Only response status is recorded;
paths, query strings, credentials, headers and identity are not span attributes.
An empty OTEL_EXPORTER_OTLP_ENDPOINT means no export (trace IDs still exist).
Set an OTLP/HTTP collector endpoint to enable batch export. Bootstrap shuts down
the provider after HTTP draining using its shared cleanup deadline. No collector
is deployed automatically and no external endpoint is configured by default.

Metrics use atomic counters with bounded status-class labels, cumulative duration
and in-flight request count. No user/request IDs or raw paths are metric labels.
GET /metrics exposes Prometheus text only in development/test. It is absent from
staging/production routes; inject the same Metrics handler into a protected route
or private management listener when configuring production monitoring. There is
no Prometheus server installed. Metrics describe completed HTTP work, including
timeouts; timed-out goroutines are not counted as active HTTP responses.

## Validation

Unit tests cover CORS allow/deny/preflight, authentication before authorization,
fail-closed wiring, concurrent rate limiting/refill, stack trace privacy,
timeout/late writes, status counters, trace propagation/log correlation and config
scope. Existing lifecycle tests cover graceful drain and forced shutdown.

References: [Go TimeoutHandler](https://pkg.go.dev/net/http#TimeoutHandler),
[OpenTelemetry Go](https://opentelemetry.io/docs/languages/go/).

Dependency pins: OpenTelemetry API/SDK/OTLP HTTP exporter v1.44.0; transitive
gRPC v1.83.1 is pinned for security fixes. go.mod/go.sum capture the exact graph.
