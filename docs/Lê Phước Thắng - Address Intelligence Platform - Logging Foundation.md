# Logging foundation

Runtime logs use `log/slog`, one JSON object per line on stdout. Construct the
logger in bootstrap through `internal/platform/logging.New`; never change
`slog.Default`. Entry-point failures use `bootstrap.LogFailure` once, including
configuration failures. The separate fallback logger is intentional because
validated configuration may not exist yet.

| Field | Scope and type |
| --- | --- |
| `service` | Every event: `api`, `worker`, or `indexer` |
| `environment` | Every event: configured `APP_ENV` (development by default) |
| `version` | Every event: entry-point build version, `dev` for local builds |
| `request_id` | Request events: server-generated 128-bit identifier, returned in `X-Request-ID` |
| `trace_id` | Only when supplied by a telemetry adapter from an active span |
| `method`, `path` | HTTP completion: method and URL path, without query string |
| `status_code` | HTTP completion: final numeric status; 0 if aborted before headers |
| `duration_ms` | HTTP completion: numeric elapsed milliseconds, including fractions |
| `error_code` | Failed events only: stable machine-readable code |

The JSON handler also includes `time`, `level`, and `msg`. Do not repeat reserved
fields in business attributes. Correlation fields remain at the JSON root even
with `WithGroup`. HTTP 4xx uses WARN, 5xx ERROR, otherwise INFO. Request aborts
use ERROR and `request_aborted`; the HTTP server recovery aborts a panic after response commitment using
http.ErrAbortHandler, preserving the status already written. A panic before
commitment becomes a safe JSON 500 and a completed ERROR access event. Panic
values are not logged because they may contain secrets. This policy is applied
by platform/httpserver; the logging middleware alone does not recover panics.

HTTP access error codes (`http_client_error`, `http_server_error`) are transport
categories, not domain error codes. Business events should use specific stable
codes where needed. Return/wrap errors in lower layers; log details once at the
boundary that handles the error. Access logs record the outcome without repeating
the error payload.

## Injection and context

Inject `*slog.Logger` into application constructors, as allowed by the dependency
policy. A consumer-owned small interface is optional if the consumer needs it;
there is no need to wrap all of slog. Domain and contract packages do not log.
The application does not import platform/logging.

```go
func (s *Service) Search(ctx context.Context, query string) error {
    // s.logger was injected by bootstrap. Keep the request context.
    s.logger.InfoContext(ctx, "address searched", "query_length", len(query))
    // ...
    return nil
}
```

Use `InfoContext` / `ErrorContext` with the original context for request events;
`Info` does not carry request correlation. Telemetry middleware, when implemented,
must call `logging.WithTraceID` with the active span's trace identifier before
the logging middleware runs. This foundation does not implement tracing or trust
incoming `traceparent` / `X-Request-ID` as server correlation.

Never log raw address queries, request bodies, authorization/cookie headers,
credentials, connection strings, or full config objects. URL paths are logged;
routes must not embed sensitive values in paths. Review error messages before
logging them: generic automatic redaction cannot reliably detect every secret.

Use `http.NewResponseController(w)` for streaming, flushing, hijacking and
deadlines through the response wrapper. Legacy optional writer type assertions
(e.g. `w.(http.Flusher)`) are not the middleware contract.

Build version uses the existing `-ldflags "-X main.version=..."` mechanism. Keep
request-specific fields absent on startup/background events; do not manufacture
empty IDs or zero HTTP fields to force every event into the same shape.

Current middleware behavior (panic stack traces, CORS, timeout, rate limiting,
tracing and metrics) is documented in [Middleware Foundation](middleware.md).
