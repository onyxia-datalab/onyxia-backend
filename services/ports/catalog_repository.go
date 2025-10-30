package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

type PackageRepository interface {
	// Lists all packages for a given catalog.
	// For Helm: reads index.yaml
	// For OCI: uses cfg.Packages	ListPackages(ctx context.Context, cfg env.CatalogConfig) ([]domain.PackageRef, error)
	ListPackages(ctx context.Context, cfg env.CatalogConfig) ([]domain.Package, error)

	// 2. Heavyweight: fetch full details for a specific package
	GetPackage(
		ctx context.Context,
		cfg env.CatalogConfig,
		name string,
	) (*domain.PackageRef, error)

	ResolvePackage(
		ctx context.Context,
		catalogID string,
		packageName string,
		version string,
	) (domain.PackageVersion, error)
}
