package usecase

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

type Catalog struct {
	envCatalogConfig []env.CatalogConfig
	//packageRep port.
}

var _ domain.CatalogService = (*Catalog)(nil)

func NewCatalogService(envCatalogConfig []env.CatalogConfig) *Catalog {
	return &Catalog{
		envCatalogConfig: envCatalogConfig,
	}
}

func (uc *Catalog) ListPublicCatalogs(ctx context.Context) ([]domain.Catalog, error) {
	var catalog = make([]domain.Catalog, 0)

	return catalog, nil
}

func (uc *Catalog) ListUserCatalog(ctx context.Context) ([]domain.Catalog, error) {

	var catalog = make([]domain.Catalog, 0)

	return catalog, nil
}
