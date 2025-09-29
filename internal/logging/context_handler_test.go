package logging

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
)

// stubHandler is a minimal slog.Handler used to capture records passed through
// the contextAttrHandler. It implements Enabled/Handle/WithAttrs/WithGroup.
// It does not aim to be a full handler; it's sufficient for unit tests.
type stubHandler struct {
	enabled    bool
	enabledCnt int

	lastRecord slog.Record
	lastAttrs  []slog.Attr
}

func (s *stubHandler) Enabled(ctx context.Context, level slog.Level) bool {
	s.enabledCnt++
	return s.enabled
}

func (s *stubHandler) Handle(ctx context.Context, r slog.Record) error {
	// Capture a copy of the record and its attributes.
	s.lastRecord = r
	attrs := make([]slog.Attr, 0)
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})
	s.lastAttrs = attrs
	return nil
}

func (s *stubHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return s }
func (s *stubHandler) WithGroup(name string) slog.Handler       { return s }

func TestNewContextAttrsHandlerReturnsBaseWhenNil(t *testing.T) {
	base := &stubHandler{}
	h := newContextAttrsHandler(base, nil)
	if h != base {
		t.Fatalf("expected base handler to be returned when attrFn is nil")
	}
}

func TestHandleAddsAttributesFromAttrFunc(t *testing.T) {
	base := &stubHandler{enabled: true}
	attrFn := func(ctx context.Context) []slog.Attr {
		return []slog.Attr{
			slog.String("username", "alice"),
			slog.Any("groups", []string{"dev", "ops"}),
		}
	}
	h := newContextAttrsHandler(base, attrFn)

	rec := slog.Record{}
	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	expected := []slog.Attr{
		slog.String("username", "alice"),
		slog.Any("groups", []string{"dev", "ops"}),
	}
	if !reflect.DeepEqual(base.lastAttrs, expected) {
		t.Fatalf("unexpected attrs: got=%v want=%v", base.lastAttrs, expected)
	}
}

func TestHandleNoAttributesWhenAttrFuncReturnsEmpty(t *testing.T) {
	base := &stubHandler{enabled: true}
	attrFn := func(ctx context.Context) []slog.Attr { return nil }
	h := newContextAttrsHandler(base, attrFn)

	rec := slog.Record{}
	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if len(base.lastAttrs) != 0 {
		t.Fatalf("expected no attrs, got %v", base.lastAttrs)
	}
}

func TestEnabledDelegatesToNext(t *testing.T) {
	base := &stubHandler{enabled: false}
	h := newContextAttrsHandler(base, func(ctx context.Context) []slog.Attr { return nil })

	if got := h.Enabled(context.Background(), slog.LevelInfo); got {
		t.Fatalf("expected Enabled=false from base handler")
	}
	if base.enabledCnt == 0 {
		t.Fatalf("expected base.Enabled to be called")
	}

	base.enabled = true
	if got := h.Enabled(context.Background(), slog.LevelInfo); !got {
		t.Fatalf("expected Enabled=true from base handler after flip")
	}
}

func TestWithAttrsPreservesAttrFunc(t *testing.T) {
	base := &stubHandler{enabled: true}
	attrFn := func(ctx context.Context) []slog.Attr { return []slog.Attr{slog.String("k", "v")} }
	wrap := newContextAttrsHandler(base, attrFn)

	h2 := wrap.WithAttrs([]slog.Attr{slog.String("x", "y")})
	inner, ok := h2.(*contextAttrHandler)
	if !ok {
		t.Fatalf("expected *contextAttrHandler after WithAttrs, got %T", h2)
	}
	if reflect.ValueOf(inner.attrFn).Pointer() != reflect.ValueOf(attrFn).Pointer() {
		t.Fatalf("attrFn was not preserved across WithAttrs")
	}
}

func TestWithGroupPreservesAttrFunc(t *testing.T) {
	base := &stubHandler{enabled: true}
	attrFn := func(ctx context.Context) []slog.Attr { return []slog.Attr{slog.String("k", "v")} }
	wrap := newContextAttrsHandler(base, attrFn)

	h2 := wrap.WithGroup("grp")
	inner, ok := h2.(*contextAttrHandler)
	if !ok {
		t.Fatalf("expected *contextAttrHandler after WithGroup, got %T", h2)
	}
	if reflect.ValueOf(inner.attrFn).Pointer() != reflect.ValueOf(attrFn).Pointer() {
		t.Fatalf("attrFn was not preserved across WithGroup")
	}
}
