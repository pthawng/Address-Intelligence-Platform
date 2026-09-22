# Error Foundation

## Review baseline

The project currently has health endpoints and module skeletons. There were no
business error identities, shared HTTP mapper, or error response contract. The
standard ServeMux returned plain-text routing errors. No existing business
handlers or repository implementations needed migration.

## Contract

Core layers use `internal/platform/apperror`, which depends only on pure standard
library packages. Dependency checks allow this explicit exception for domain,
application and contract while continuing to prohibit HTTP dependencies there.
`internal/platform/httpserver` owns status codes, public messages and JSON encoding.
No third-party dependencies are added.

| Error code | HTTP |
| --- | --- |
| ADDRESS_NOT_FOUND | 404 |
| INVALID_ADDRESS | 400 |
| INVALID_COORDINATES | 400 |
| PROVIDER_UNAVAILABLE | 503 |
| RATE_LIMIT_EXCEEDED | 429 |
| INTERNAL_ERROR | 500 |
| ROUTE_NOT_FOUND (transport only) | 404 |
| METHOD_NOT_ALLOWED (transport only) | 405 |

```json
{"success":false,"status":404,"error":{"code":"ADDRESS_NOT_FOUND","message":"Address not found","request_id":"server-generated-id"}}
```

Clients branch on `code`, not `message`. Messages are fixed and public. Unknown
errors/codes become INTERNAL_ERROR; raw error text and causes are never serialized.
Responses use application/json, no-store and nosniff. HEAD has no error body.
The request ID comes from logging context, matches X-Request-ID, and is omitted
when no context ID exists. Incoming IDs are not trusted.

## Usage

```go
// Domain/application: immutable sentinel, usable with errors.Is.
return apperror.ErrAddressNotFound

// Adapter/application: classify while preserving a diagnostic cause.
return &apperror.AppError{
    Code: apperror.ErrProviderUnavailable,
    Err: fmt.Errorf("geocode provider: %w", err),
}

// Transport: one adapter owns error responses across endpoints.
mux.Handle("GET /addresses/{id}", httpserver.Adapt(handler.Get))
```

Handler signature: `func(http.ResponseWriter, *http.Request) error`. Return errors
before writing headers/body; write successful responses normally and return nil.
For existing handler signatures, call `httpserver.WriteError(w, r, err)` and return.
Never send AppError directly to a JSON encoder or use http.Error in endpoints.

`errors.Is` preserves both classification and cause identity; `errors.As` retrieves
AppError. Wrap with `%w`. CodeOf uses the outermost classification, so an internal
error wrapping a not-found cause remains HTTP 500. For errors.Join the first
classification in depth-first order wins; explicitly classify aggregates when
that ordering is not the intended public meaning. Nil is a no-op for WriteError;
typed nil and zero/unknown AppError codes fail closed to 500.

Translate driver/provider errors at the adapter/application boundary, where their
meaning is known. Do not globally map database not-found, context cancellation or
timeout to a business error. Add a new code and mapper case with contract tests
when new semantics are needed. Retry-After is not fabricated: set it only when a
rate limiter/provider supplies a reliable retry interval.

Use `httpserver.NewRouter` and `router.Handle` for future routes. The router
automatically maps unmatched paths and methods to JSON 404/405 and preserves
ServeMux Allow headers and canonical-path redirects. Protocol parsing errors produced by net/http
before dispatch are outside this handler contract.

Existing logging middleware records one completion event; mapper does not log raw
causes, which can contain credentials or personal data. If diagnosis requires a
business event, log safe attributes once at the handling boundary with the request
context. The HTTP server middleware converts panics before response commitment to a safe
500. After commitment it aborts the request without appending another response.
See [HTTP Server Foundation](Lê%20Phước%20Thắng%20-%20Address%20Intelligence%20Platform%20-%20HTTP%20Server%20Foundation.md).

## Validation

Tests cover all six mappings, wrapping, errors.Is/As, outer classification,
unknown/typed-nil errors, cause non-disclosure, correlation, HEAD, nil/success
responses, actual health routing 404/405 and dependency boundaries.

The current envelope, validation details and 401/403 mappings are specified in
[Standard API Response](Lê%20Phước%20Thắng%20-%20Address%20Intelligence%20Platform%20-%20Standard%20API%20Response.md).
