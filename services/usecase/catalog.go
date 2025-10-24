package usecase

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/internal/utils"
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

func (uc *Catalog) listOCICatalog(ctx context.Context, cfg env.CatalogConfig) domain.Catalog {
	return domain.Catalog{
		ID:                  cfg.ID,
		Name:                utils.NewLocalizedString(cfg.Name),
		Description:         utils.NewLocalizedString(cfg.Description),
		Status:              cfg.Status,
		HighlightedPackages: cfg.Highlighted,
		Visible: domain.CatalogVisibility{
			User:    cfg.Visible.User,
			Project: cfg.Visible.Project,
		},
		Packages: func() []domain.Package {
			pkgs := make([]domain.Package, 0, len(cfg.Packages))
			for _, p := range cfg.Packages {
				pkgs = append(pkgs, domain.Package{Name: p.Name})
			}
			return pkgs
		}(),
	}
}
