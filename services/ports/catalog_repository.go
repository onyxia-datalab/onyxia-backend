package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

type CatalogRepository interface {
	ListPackages(ctx context.Context, cfg env.CatalogConfig) ([]domain.PackageRef, error)
}
