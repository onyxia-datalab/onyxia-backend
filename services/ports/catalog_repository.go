package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

type PackageRepository interface {
	// Helm repo add <catalog> puis helm search repo <catalog>
	// ou avec la config pour les OCI
	ListPackages(ctx context.Context, catalogID string) ([]domain.Package, error)
	//helm search repo <catalog>/<package>
	GetPackage(ctx context.Context, catalogID string, name string) (domain.Package, error)
	// helm search repo <catalog>/<package> --versions
	// avec la config pour les OCI,
	GetAvailableVersions(ctx context.Context, catalogID string, name string) ([]string, error)
	// 	helm pull <repo>/<chart> --version <version>
	// tar -xf <chart>-<version>.tgz <chart>/values.schema.json
	// cat <chart>/values.schema.json
	GetPackageSchema(
		ctx context.Context,
		catalogID string,
		packageName string,
		version string,
	) ([]byte, error)
}
