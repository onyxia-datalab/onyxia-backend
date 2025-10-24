package usecase

import (
	"context"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/internal/tools"
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

// nolint:unused
func (uc *Catalog) listOCICatalog(ctx context.Context, cfg env.CatalogConfig) domain.Catalog {

	name, err := tools.NewLocalizedString(cfg.Name)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse catalog name",
			"catalog_id", cfg.ID,
			"error", err,
		)
		name = tools.LocalizedString{}
	}
	description, err := tools.NewLocalizedString(cfg.Description)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse catalog description",
			"catalog_id", cfg.ID,
			"error", err,
		)
		description = tools.LocalizedString{}
	}

	return domain.Catalog{
		ID:                  cfg.ID,
		Name:                name,
		Description:         description,
		Status:              domain.CatalogStatus(cfg.Status),
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
