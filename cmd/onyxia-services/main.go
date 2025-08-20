package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v3"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/onyxia-datalab/onyxia-backend/services/api/route"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
)

func main() {

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.NewApplication(ctx)
	if err != nil {
		slog.Error("failed to initialize application",
			slog.Any("error", err),
		)
		os.Exit(1)
	}

	env := app.Env

	r := chi.NewRouter()

	r.Use(middleware.Heartbeat("/healthz"))

	logger := slog.Default()

	r.Use(
		httplog.RequestLogger(logger, &httplog.Options{Level: slog.LevelInfo, RecoverPanics: true}),
	)

	r.Use(middleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: env.Security.CORSAllowedOrigins,
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"X-CSRF-Token",
			"Origin",
			"X-Requested-With",
			"onyxia-region",
		},
		ExposedHeaders:   []string{"Link", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	apiHandler, err := route.Setup(ctx, app)

	if err != nil {
		slog.Error("failed to set up routes", slog.Any("error", err))
		os.Exit(1)
	}

	r.Mount(
		env.Server.ContextPath,
		http.StripPrefix(env.Server.ContextPath, apiHandler),
	)

	slog.Info("API mounted", slog.String("contextPath", env.Server.ContextPath))

	address := fmt.Sprintf(":%d", env.Server.Port)

	slog.Info("Server starting...", slog.String("address", address))

	if err := http.ListenAndServe(address, r); err != nil {
		slog.Error("failed to listen and serve",
			slog.Any("error", err),
		)
		os.Exit(1)
	}

}
