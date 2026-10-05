// Package server holds the HTTP plumbing shared by every API: the middleware
// stack in front of the generated handlers and the server lifecycle,
// including graceful shutdown.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v3"
	"github.com/onyxia-datalab/onyxia-backend/internal/httputil"
)

// Config is the `server` section of an API's configuration.
type Config struct {
	Port int `mapstructure:"port" json:"port"`
	// ShutdownTimeout bounds the whole graceful shutdown: draining in-flight
	// requests, then background work. Keep it below the pod's
	// terminationGracePeriodSeconds (30s by default).
	ShutdownTimeout time.Duration `mapstructure:"shutdownTimeout" json:"shutdownTimeout"`
}

// DrainFunc waits for an API's background work to finish. It must return
// when ctx is done, even if the work is still running.
type DrainFunc func(ctx context.Context) error

// Handler wraps an API handler with the middleware stack shared by every API.
func Handler(api http.Handler, corsAllowedOrigins []string) http.Handler {
	r := chi.NewRouter()

	r.Use(httplog.RequestLogger(slog.Default(), &httplog.Options{
		Level:         slog.LevelInfo,
		RecoverPanics: true,
	}))
	r.Use(middleware.Heartbeat("/healthz"))
	r.Use(httputil.ProxyHeaders)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: corsAllowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{
			"Accept",
			"Authorization",
			"Content-Type",
			"DPoP",
			"X-CSRF-Token",
			"Origin",
			"X-Requested-With",
			"onyxia-region",
			"X-Onyxia-Project",
		},
		ExposedHeaders:   []string{"Link", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Mount("/", api)
	return r
}

// Run serves handler until ctx is done, then shuts down gracefully: it stops
// accepting connections, waits for in-flight requests to complete, and then
// for each drain function — all within cfg.ShutdownTimeout.
//
// Drains run after the HTTP shutdown so that work started by the last
// requests (e.g. an asynchronous Helm install) is waited for too.
func Run(ctx context.Context, cfg Config, handler http.Handler, drains ...DrainFunc) error {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	return serve(ctx, ln, cfg.ShutdownTimeout, handler, drains...)
}

func serve(
	ctx context.Context,
	ln net.Listener,
	shutdownTimeout time.Duration,
	handler http.Handler,
	drains ...DrainFunc,
) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		// No ReadTimeout/WriteTimeout: they would cut long-lived SSE streams.
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	slog.InfoContext(ctx, "Server started", slog.String("address", ln.Addr().String()))

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	slog.InfoContext(ctx, "Shutting down", slog.Duration("timeout", shutdownTimeout))
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	var errs []error
	if err := srv.Shutdown(shutdownCtx); err != nil {
		errs = append(errs, fmt.Errorf("http shutdown: %w", err))
	}
	for _, drain := range drains {
		if err := drain(shutdownCtx); err != nil {
			errs = append(errs, err)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}

	slog.InfoContext(ctx, "Shutdown complete")
	return nil
}
