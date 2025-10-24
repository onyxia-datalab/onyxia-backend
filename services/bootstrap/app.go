package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/internal/kube"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
)

type Application struct {
	Env               *env.Env
	K8sClient         *kube.Client
	UserContextReader usercontext.Reader
	UserContextWriter usercontext.Writer
}

func NewApplication(ctx context.Context) (*Application, error) {
	userReader, userWriter := usercontext.NewUserContext()

	InitLogger(userReader)

	env, err := env.New()
	if err != nil {
		return nil, fmt.Errorf("failed to load environment: %w", err)

	}

	k8sClient, err := kube.NewClient("")

	if err != nil {
		return nil, fmt.Errorf("failed to initialize Kubernetes client: %w", err)
	}

	if err := k8sClient.Ping(ctx); err != nil {
		slog.ErrorContext(ctx, "failed to reach Kubernetes API", "error", err)
		return nil, fmt.Errorf("failed to reach Kubernetes API: %w", err)
	}
	
	app := &Application{
		Env:               &env,
		K8sClient:         k8sClient,
		UserContextReader: userReader,
		UserContextWriter: userWriter,
	}

	slog.Info("Application initialized successfully")

	return app, nil
}
