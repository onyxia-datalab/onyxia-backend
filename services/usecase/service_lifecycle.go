// services/usecase/service_lifecycle.go
package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

type ServiceLifecycle struct {
	secrets ports.OnyxiaSecretGateway
	helm    ports.HelmReleasesGateway
}

var _ domain.ServiceLifecycle = (*ServiceLifecycle)(nil)

func NewServiceLifecycle(
	secrets ports.OnyxiaSecretGateway,
	helm ports.HelmReleasesGateway,
) *ServiceLifecycle {
	return &ServiceLifecycle{secrets: secrets, helm: helm}
}

func (uc *ServiceLifecycle) Start(
	ctx context.Context,
	req domain.StartRequest,
) (domain.StartResponse, error) {

	// 1) Secret Onyxia
	secretData := map[string][]byte{
		// TODO
	}
	if err := uc.secrets.EnsureOnyxiaSecret(ctx, req.Namespace, req.Name, secretData); err != nil {
		return domain.StartResponse{}, fmt.Errorf("create onyxia secret: %w", err)
	}

	opts := ports.HelmStartOptions{
		Callbacks: ports.HelmStartCallbacks{
			OnStart: func(release, chart string) {
				slog.InfoContext(ctx, "helm install started",
					"release", release, "chart", chart, "namespace", req.Namespace)
			},
			OnSuccess: func(release, chart string) {
				slog.InfoContext(ctx, "helm install succeeded",
					"release", release, "chart", chart, "namespace", req.Namespace)
			},
			OnError: func(release, chart string, err error) {
				slog.ErrorContext(ctx, "helm install failed",
					"release", release, "chart", chart, "namespace", req.Namespace, "err", err)
			},
		},
	}

	if err := uc.helm.StartInstall(ctx, req.Name, req.Chart, req.Values, opts); err != nil {
		return domain.StartResponse{}, fmt.Errorf("helm start: %w", err)
	}

	return domain.StartResponse{}, nil
}

func (uc *ServiceLifecycle) Resume(ctx context.Context) error {
	return nil
}

func (uc *ServiceLifecycle) Delete(ctx context.Context) error {
	return nil
}

func (uc *ServiceLifecycle) Rename(ctx context.Context) error {
	return nil
}

func (uc *ServiceLifecycle) Share(ctx context.Context) error {
	return nil
}
