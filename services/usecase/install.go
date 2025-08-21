package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/infrastructure/helm"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

type installUsecase struct {
	k8s  ports.KubernetesService
	helm ports.Helm
}

func NewInstallUsecase(
	k8s ports.KubernetesService,
	helm ports.Helm,
) *installUsecase {
	return &installUsecase{
		k8s: k8s, helm: helm,
	}
}

func (s *installUsecase) Install(
	ctx context.Context,
	req domain.InstallRequest,
) (domain.InstallResponse, error) {

	// create Onyxia secret
	if err := s.k8s.CreateOnyxiaSecret(ctx, req.Namespace, req.SecretName, req.SecretData); err != nil {
		return domain.InstallResponse{}, fmt.Errorf("create onyxia secret: %w", err)
	}

	// start Helm install
	opts := helm.InstallOptions{
		Callbacks: helm.Callbacks{
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

	if err := s.helm.StartInstall(ctx, req.ReleaseName, req.Chart, req.Values, opts); err != nil {
		// pre-flight failure (bad chart, values merge, locate error, etc.)
		return domain.InstallResponse{}, fmt.Errorf("helm start: %w", err)
	}

	return domain.InstallResponse{}, nil

}
