package logging

import (
	"context"
	"log/slog"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/exp/zapslog"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// helper: build an observed zap logger at the given level
func observedZap(level zapcore.Level) (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(level)
	return zap.New(core), logs
}

func TestNewLoggerUsesProvidedZapAndEnrichesFromContext(t *testing.T) {
	zl, logs := observedZap(zapcore.InfoLevel)

	attrFn := func(ctx context.Context) []slog.Attr {
		return []slog.Attr{slog.String("username", "bob")}
	}

	logger, flush, err := NewLogger(zl, attrFn, zapslog.WithCaller(false))
	if err != nil {
		t.Fatalf("NewLogger returned error: %v", err)
	}
	t.Cleanup(func() { _ = flush() })

	ctx := context.Background()
	logger.InfoContext(ctx, "hello", slog.String("extra", "x"))

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	entry := logs.All()[0]

	// assert logger saw our message
	if entry.Message != "hello" {
		t.Fatalf("unexpected message: got %q", entry.Message)
	}

	// assert fields include username and extra
	got := map[string]any{}
	for _, f := range entry.Context {
		switch f.Type {
		case zapcore.StringType:
			got[f.Key] = f.String
		case zapcore.StringerType:
			got[f.Key] = f.Interface
		case zapcore.Int64Type:
			got[f.Key] = f.Integer
		case zapcore.ArrayMarshalerType, zapcore.ObjectMarshalerType, zapcore.ReflectType:
			got[f.Key] = f.Interface
		case zapcore.BoolType:
			got[f.Key] = f.Integer == 1
		default:
			if f.Interface != nil {
				got[f.Key] = f.Interface
			}
		}
	}

	if got["username"] != "bob" {
		t.Fatalf("missing or wrong username: got=%v", got["username"])
	}
	if got["extra"] != "x" {
		t.Fatalf("missing or wrong extra: got=%v", got["extra"])
	}
}

func TestNewLoggerAppliesHandlerOptionsNameAndCaller(t *testing.T) {
	zl, logs := observedZap(zapcore.InfoLevel)

	logger, flush, err := NewLogger(zl, nil,
		zapslog.WithName("onboarding"),
		zapslog.WithCaller(true),
	)
	if err != nil {
		t.Fatalf("NewLogger returned error: %v", err)
	}
	t.Cleanup(func() { _ = flush() })

	logger.Info("ping")
	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	entry := logs.All()[0]

	if entry.LoggerName != "onboarding" {
		t.Fatalf("expected logger name 'onboarding', got %q", entry.LoggerName)
	}
	// We don't assert caller content (depends on build flags), but we at least assert it exists
	if !entry.Caller.Defined {
		t.Fatalf("expected caller to be defined when WithCaller(true) is set")
	}
}

func TestNewLoggerFlushDoesNotErrorWithObserver(t *testing.T) {
	zl, _ := observedZap(zapcore.InfoLevel)
	logger, flush, err := NewLogger(zl, nil)
	if err != nil {
		t.Fatalf("NewLogger returned error: %v", err)
	}
	_ = logger // not used further; we only care that flush is callable
	if err := flush(); err != nil {
		t.Fatalf("flush returned error: %v", err)
	}
}
