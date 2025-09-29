package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

type PackageResolver interface {
	ResolvePackage(
		ctx context.Context,
		catalogID string,
		packageName string,
		version string,
	) (domain.PackageVersion, error)
}
