package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/internal/kube"
	"github.com/onyxia-datalab/onyxia-backend/internal/logging"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
)

type Application struct {
	Env               *env.Env
	K8sClient         *kube.Client
	UserContextReader usercontext.Reader
	UserContextWriter usercontext.Writer
	// FlushLogs flushes buffered log records. Call it before the process exits.
	FlushLogs func() error
}

func NewApplication(ctx context.Context) (*Application, error) {
	userReader, userWriter := usercontext.NewUserContext()

	flushLogs, err := logging.SetupDefault(userReader)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	env, err := env.New()
	if err != nil {
		return nil, fmt.Errorf("failed to load environment: %w", err)

	}

	k8sClient, err := kube.NewClient("")

	if err != nil {
		return nil, fmt.Errorf("failed to initialize Kubernetes client: %w", err)
	}

	if err := k8sClient.Ping(ctx); err != nil {
		slog.ErrorContext(ctx, "failed to reach Kubernetes API", slog.Any("error", err))
		return nil, fmt.Errorf("failed to reach Kubernetes API: %w", err)
	}

	app := &Application{
		Env:               &env,
		K8sClient:         k8sClient,
		UserContextReader: userReader,
		UserContextWriter: userWriter,
		FlushLogs:         flushLogs,
	}

	slog.InfoContext(ctx, "Application initialized successfully")

	return app, nil
}
