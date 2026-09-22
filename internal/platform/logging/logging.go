// Package logging constructs structured loggers for runtime injection.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Options describes process identity. Construct loggers only at composition roots.
type Options struct {
	Service     string
	Environment string
	Version     string
	Level       slog.Level
	Output      io.Writer
}

func New(opts Options) *slog.Logger {
	if opts.Output == nil {
		opts.Output = os.Stdout
	}
	if opts.Version == "" {
		opts.Version = "dev"
	}
	handler := &contextHandler{Handler: slog.NewJSONHandler(opts.Output, &slog.HandlerOptions{Level: opts.Level})}
	return slog.New(handler).With("service", opts.Service, "environment", opts.Environment, "version", opts.Version)
}

type correlationKey struct{}
type correlation struct{ requestID, traceID string }

// WithRequestID attaches a server-generated request identifier.
func WithRequestID(ctx context.Context, id string) context.Context {
	c, _ := ctx.Value(correlationKey{}).(correlation)
	c.requestID = id
	return context.WithValue(ctx, correlationKey{}, c)
}

// WithTraceID is for the telemetry adapter: pass the active span's trace ID,
// never a raw incoming header. No trace ID is fabricated when tracing is absent.
func WithTraceID(ctx context.Context, id string) context.Context {
	c, _ := ctx.Value(correlationKey{}).(correlation)
	c.traceID = id
	return context.WithValue(ctx, correlationKey{}, c)
}

type handlerOperation struct {
	attrs []slog.Attr
	group string
}

type contextHandler struct {
	slog.Handler
	operations []handlerOperation
}

func (h *contextHandler) Handle(ctx context.Context, record slog.Record) error {
	c, _ := ctx.Value(correlationKey{}).(correlation)
	var attrs []slog.Attr
	if c.requestID != "" {
		attrs = append(attrs, slog.String("request_id", c.requestID))
	}
	if c.traceID != "" {
		attrs = append(attrs, slog.String("trace_id", c.traceID))
	}
	// Attach correlation before replaying groups so the schema stays at the root.
	handler := h.Handler.WithAttrs(attrs)
	for _, op := range h.operations {
		if op.group != "" {
			handler = handler.WithGroup(op.group)
		} else {
			handler = handler.WithAttrs(op.attrs)
		}
	}
	return handler.Handle(ctx, record)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Resolve values at With time, matching slog's handler contract.
	resolved := make([]slog.Attr, len(attrs))
	for i, attr := range attrs {
		resolved[i] = resolveAttr(attr)
	}
	return h.withOperation(handlerOperation{attrs: resolved})
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return h.withOperation(handlerOperation{group: name})
}

func (h *contextHandler) withOperation(op handlerOperation) *contextHandler {
	ops := make([]handlerOperation, len(h.operations), len(h.operations)+1)
	copy(ops, h.operations)
	return &contextHandler{Handler: h.Handler, operations: append(ops, op)}
}

func resolveAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	if attr.Value.Kind() == slog.KindGroup {
		group := attr.Value.Group()
		resolved := make([]slog.Attr, len(group))
		for i, child := range group {
			resolved[i] = resolveAttr(child)
		}
		attr.Value = slog.GroupValue(resolved...)
	}
	return attr
}

// Failure reports an entry-point failure even when configuration could not load.
func Failure(service, version string, err error) {
	environment := os.Getenv("APP_ENV")
	if environment == "" {
		environment = "development"
	}
	logger := New(Options{Service: service, Environment: environment, Version: version})
	logger.Error("runtime failed", "error_code", "runtime_failed", "error", err)
}

// RequestID returns the server-generated ID, or empty outside HTTP.
func RequestID(ctx context.Context) string {
	c, _ := ctx.Value(correlationKey{}).(correlation)
	return c.requestID
}
