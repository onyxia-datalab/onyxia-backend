// services/usecase/service_lifecycle.go
package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

type ServiceLifecycle struct {
	secrets ports.OnyxiaSecretGateway
	helm    ports.HelmReleasesGateway
	pkgRepo ports.PackageRepository
}

var _ domain.ServiceLifecycle = (*ServiceLifecycle)(nil)

func NewServiceLifecycle(
	secrets ports.OnyxiaSecretGateway,
	helm ports.HelmReleasesGateway,
	pkgRepo ports.PackageRepository,
) *ServiceLifecycle {
	return &ServiceLifecycle{secrets: secrets, helm: helm, pkgRepo: pkgRepo}
}

func (uc *ServiceLifecycle) Start(
	ctx context.Context,
	req domain.StartRequest,
) (domain.StartResponse, error) {

	// 1) Get the package from catalog
	pkg, err := uc.pkgRepo.GetPackage(ctx, req.CatalogID, req.PackageName)
	if err != nil {
		return domain.StartResponse{}, fmt.Errorf("get package: %w", err)
	}

	// 2) Create the Onyxia Secret
	secretData := map[string][]byte{
		"catalog":      []byte(req.CatalogID),
		"friendlyName": []byte(req.FriendlyName),
		"owner":        []byte(req.Username),
		"share":        []byte(strconv.FormatBool(req.Share)),
	}

	if err := uc.secrets.EnsureOnyxiaSecret(ctx, req.Namespace, req.ReleaseID, secretData); err != nil {
		return domain.StartResponse{}, fmt.Errorf("create onyxia secret: %w", err)
	}

	// 3) Start the helm install
	opts := ports.HelmStartOptions{
		Callbacks: ports.HelmStartCallbacks{
			OnStart: func(release, chart string) {
				slog.InfoContext(ctx, "helm install started",
					slog.String("release", release),
					slog.String("chart", chart),
					slog.String("namespace", req.Namespace),
				)
			},
			OnSuccess: func(release, chart string) {
				slog.InfoContext(ctx, "helm install succeeded",
					slog.String("release", release),
					slog.String("chart", chart),
					slog.String("namespace", req.Namespace),
				)
			},
			OnError: func(release, chart string, err error) {
				slog.ErrorContext(ctx, "helm install failed",
					slog.String("release", release),
					slog.String("chart", chart),
					slog.String("namespace", req.Namespace),
					slog.Any("error", err),
				)
			},
		},
	}

	if err := uc.helm.StartInstall(ctx, req.Namespace, req.Name, &pkg, req.Version, req.Values, opts); err != nil {
		return domain.StartResponse{}, fmt.Errorf("helm start: %w", err)
	}

	return domain.StartResponse{}, nil
}

func (uc *ServiceLifecycle) Suspend(ctx context.Context, req domain.SuspendRequest) error {
	return uc.helm.SuspendRelease(ctx, req.Namespace, req.ReleaseName)
}

func (uc *ServiceLifecycle) Resume(ctx context.Context, req domain.ResumeRequest) error {
	return uc.helm.ResumeRelease(ctx, req.Namespace, req.ReleaseName)
}

func (uc *ServiceLifecycle) Delete(ctx context.Context, req domain.DeleteRequest) error {
	if err := uc.helm.UninstallRelease(ctx, req.Namespace, req.ReleaseName); err != nil {
		return fmt.Errorf("helm uninstall: %w", err)
	}
	if err := uc.secrets.DeleteOnyxiaSecret(ctx, req.Namespace, req.ReleaseName); err != nil {
		return fmt.Errorf("delete onyxia secret: %w", err)
	}
	return nil
}

func (uc *ServiceLifecycle) Rename(ctx context.Context) error {
	return nil
}

func (uc *ServiceLifecycle) Share(ctx context.Context) error {
	return nil
}
