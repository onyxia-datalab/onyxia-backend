package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

type PackageRepository interface {
	// helm repo add <catalog>, then helm search repo <catalog>
	// or using the appropriate configuration for OCI registries
	ListPackages(ctx context.Context, catalogID string) ([]domain.Package, error)

	// helm search repo <catalog>/<package>
	GetPackage(ctx context.Context, catalogID string, name string) (domain.Package, error)

	// helm search repo <catalog>/<package> --versions
	// or using the appropriate configuration for OCI registries
	GetAvailableVersions(ctx context.Context, catalogID string, name string) ([]string, error)

	// helm pull <repo>/<chart> --version <version>
	// tar -xf <chart>-<version>.tgz <chart>/values.schema.json
	// cat <chart>/values.schema.json
	GetPackageSchema(
		ctx context.Context,
		catalogID string,
		packageName string,
		version string,
	) ([]byte, error)
}
