package logging

import (
	"context"
	"log/slog"
)

type AttrFunc func(ctx context.Context) []slog.Attr

// contextAttrHandler wraps another slog.Handler and enriches records by
// invoking an AttrFunc with the logging context. If AttrFunc returns no
// attributes or is nil, the record is passed through unchanged.
type contextAttrHandler struct {
	next   slog.Handler
	attrFn AttrFunc
}

var _ slog.Handler = (*contextAttrHandler)(nil)

func (h *contextAttrHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *contextAttrHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.attrFn != nil {
		if attrs := h.attrFn(ctx); len(attrs) > 0 {
			r.AddAttrs(attrs...)
		}
	}
	return h.next.Handle(ctx, r)
}

func (h *contextAttrHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextAttrHandler{
		next:   h.next.WithAttrs(attrs),
		attrFn: h.attrFn,
	}
}

func (h *contextAttrHandler) WithGroup(name string) slog.Handler {
	return &contextAttrHandler{
		next:   h.next.WithGroup(name),
		attrFn: h.attrFn,
	}
}

func newContextAttrsHandler(base slog.Handler, attrFn AttrFunc) slog.Handler {
	if attrFn == nil {
		return base
	}
	return &contextAttrHandler{next: base, attrFn: attrFn}
}
