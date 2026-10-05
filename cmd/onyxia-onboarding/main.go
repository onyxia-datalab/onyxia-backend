package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/onyxia-datalab/onyxia-backend/internal/server"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/api/route"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/bootstrap"
)

func main() {
	if err := run(); err != nil {
		slog.Error("onyxia-onboarding stopped with an error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	// SIGTERM (Kubernetes) and SIGINT start a graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.NewApplication(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize application: %w", err)
	}
	defer func() {
		if err := app.FlushLogs(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to flush logs: %v\n", err)
		}
	}()

	apiHandler, err := route.Setup(ctx, app)
	if err != nil {
		return fmt.Errorf("failed to set up routes: %w", err)
	}

	return server.Run(
		ctx,
		app.Env.Server,
		server.Handler(apiHandler, app.Env.Security.CORSAllowedOrigins),
	)
}
