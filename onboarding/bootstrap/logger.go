package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/onyxia-datalab/onyxia-backend/internal/logging"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/interfaces"
	"go.uber.org/zap/exp/zapslog"
)

// InitLogger initializes the global logger and handles log flushing on exit.
func InitLogger(userReaderCtx interfaces.UserContextReader) {

	attrFn := func(ctx context.Context) []slog.Attr {
		attrs := make([]slog.Attr, 0, 3)
		if u, ok := userReaderCtx.GetUsername(ctx); ok && u != "" {
			attrs = append(attrs, slog.String("username", u))
		}
		if g, ok := userReaderCtx.GetGroups(ctx); ok && len(g) > 0 {
			attrs = append(attrs, slog.Any("groups", g))
		}
		if r, ok := userReaderCtx.GetRoles(ctx); ok && len(r) > 0 {
			attrs = append(attrs, slog.Any("roles", r))
		}
		return attrs
	}

	logger, flush, err := logging.NewLogger(nil, attrFn, zapslog.WithCaller(true))
	if err != nil {
		slog.Default().Error("Failed to initialize logger", slog.Any("error", err))
		os.Exit(1)
	}
	slog.SetDefault(logger)

	// Graceful shutdown (ideally move to cmd/<service>/main.go)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		slog.Info("Flushing logs before exit...")
		if err := flush(); err != nil {
			slog.Error("Failed to flush logs", slog.Any("error", err))
		}
		os.Exit(0)
	}()
}
