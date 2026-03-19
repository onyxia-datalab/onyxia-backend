package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

type PackageRepository interface {
	ListPackages(ctx context.Context, catalogID string) ([]domain.Package, error)
	GetPackage(ctx context.Context, catalogID string, name string) (*domain.PackageRef, error)
	GetPackageSchema(ctx context.Context, catalogID string, packageName string, version string) ([]byte, error)
	ResolvePackage(ctx context.Context, catalogID string, packageName string, version string) (domain.PackageVersion, error)
}
