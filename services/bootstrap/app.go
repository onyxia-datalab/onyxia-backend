package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/internal/kube"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
)

type Application struct {
	Env               *Env
	K8sClient         *kube.Client
	UserContextReader usercontext.Reader
	UserContextWriter usercontext.Writer
}

func NewApplication(ctx context.Context) (*Application, error) {
	userReader, userWriter := usercontext.NewUserContext()

	InitLogger(userReader)

	env, err := NewEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load environment: %w", err)

	}

	k8sClient, err := kube.NewClient("")

	if err != nil {
		return nil, fmt.Errorf("failed to initialize Kubernetes client: %w", err)
	}

	_ = k8sClient.Ping(ctx)

	app := &Application{
		Env:               &env,
		K8sClient:         k8sClient,
		UserContextReader: userReader,
		UserContextWriter: userWriter,
	}

	slog.Info("Application initialized successfully")

	return app, nil
}
