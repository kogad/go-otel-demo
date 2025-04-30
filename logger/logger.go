package logger

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

type TraceHandler struct {
	inner slog.Handler
}

func NewTraceHandler(inner slog.Handler) slog.Handler {
	return &TraceHandler{inner: inner}
}

func (h *TraceHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return h.inner.Enabled(ctx, lvl)
}

func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if span := trace.SpanFromContext(ctx); span != nil {
		if sc := span.SpanContext(); sc.IsValid() {
			r.AddAttrs(
				slog.String("trace_id", sc.TraceID().String()),
				slog.String("span_id", sc.SpanID().String()),
			)
		}
	}
	return h.inner.Handle(ctx, r)
}

func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *TraceHandler) WithGroup(name string) slog.Handler {
	return &TraceHandler{inner: h.inner.WithGroup(name)}
}
