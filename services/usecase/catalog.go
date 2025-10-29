package usecase

import (
	"context"
	"fmt"
	"regexp"

	"github.com/onyxia-datalab/onyxia-backend/internal/tools"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

// Catalog implements domain.CatalogService
type Catalog struct {
	envCatalogConfig []env.CatalogConfig
	repo             ports.CatalogRepository
}

var _ domain.CatalogService = (*Catalog)(nil)

// Constructor
func NewCatalogService(
	envCatalogConfig []env.CatalogConfig,
	repo ports.CatalogRepository,
) *Catalog {
	return &Catalog{
		envCatalogConfig: envCatalogConfig,
		repo:             repo,
	}
}

func (uc *Catalog) ListPublicCatalogs(ctx context.Context) ([]domain.Catalog, error) {
	return uc.buildCatalogs(ctx, func(c env.CatalogConfig) bool {
		return len(c.Restrictions) == 0
	})
}

func (uc *Catalog) ListUserCatalog(
	ctx context.Context,
	user usercontext.Reader,
) ([]domain.Catalog, error) {
	return uc.buildCatalogs(ctx, func(c env.CatalogConfig) bool {
		if len(c.Restrictions) == 0 {
			return true
		}

		attrs, ok := user.GetAttributes(ctx)
		if !ok {
			return false
		}

		for _, r := range c.Restrictions {
			if r.UserAttributeKey == "" || r.Match == "" {
				continue
			}

			val, ok := attrs[r.UserAttributeKey]
			if !ok {
				continue
			}

			re, err := regexp.Compile(r.Match)
			if err != nil {
				continue
			}

			switch v := val.(type) {
			case string:
				if re.MatchString(v) {
					return true
				}
			case []string:
				for _, s := range v {
					if re.MatchString(s) {
						return true
					}
				}
			case []any:
				for _, s := range v {
					if str, ok := s.(string); ok && re.MatchString(str) {
						return true
					}
				}
			}
		}

		return false
	})
}

func (uc *Catalog) buildCatalogs(
	ctx context.Context,
	include func(env.CatalogConfig) bool,
) ([]domain.Catalog, error) {
	out := make([]domain.Catalog, 0)

	for _, cfg := range uc.envCatalogConfig {
		if !include(cfg) {
			continue
		}

		pkgs, err := uc.repo.ListPackages(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("catalog %q: list packages: %w", cfg.ID, err)
		}

		name, err := tools.NewLocalizedString(cfg.Name)
		if err != nil {
			name = tools.LocalizedString{} // or log it / ignore gracefully
		}

		desc, err := tools.NewLocalizedString(cfg.Description)
		if err != nil {
			desc = tools.LocalizedString{}
		}

		out = append(out, domain.Catalog{
			ID:                  cfg.ID,
			Name:                name,
			Description:         desc,
			Status:              domain.CatalogStatus(cfg.Status),
			HighlightedPackages: append([]string(nil), cfg.Highlighted...),
			Packages:            pkgs,
		})
	}

	return out, nil
}
