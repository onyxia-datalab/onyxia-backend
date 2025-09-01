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
	secrets         ports.OnyxiaSecretGateway
	helm            ports.HelmReleasesGateway
	packageResolver ports.PackageResolver
}

var _ domain.ServiceLifecycle = (*ServiceLifecycle)(nil)

func NewServiceLifecycle(
	secrets ports.OnyxiaSecretGateway,
	helm ports.HelmReleasesGateway,
	packageResolver ports.PackageResolver,
) *ServiceLifecycle {
	return &ServiceLifecycle{secrets: secrets, helm: helm, packageResolver: packageResolver}
}

func (uc *ServiceLifecycle) Start(
	ctx context.Context,
	req domain.StartRequest,
) (domain.StartResponse, error) {

	// 1) Get the package from catalog + packageName + packageVersion

	pkg, err := uc.packageResolver.ResolvePackage(ctx, req.CatalogID, req.PackageName, req.Version)

	if err != nil {
		return domain.StartResponse{}, fmt.Errorf("resolve package: %w", err)
	}

	// 2) Create the  Secret Onyxia

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

	if err := uc.helm.StartInstall(ctx, req.Name, pkg, req.Values, opts); err != nil {
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
