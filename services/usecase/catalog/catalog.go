package catalog

import (
	"context"
	"fmt"
	"slices"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

// Catalog implements ports.CatalogService
type Catalog struct {
	catalogs       []domain.CatalogSettings
	pkgRepo        ports.PackageRepository
	schemaResolver *schemaResolver
}

var _ ports.CatalogService = (*Catalog)(nil)

// Constructor
func NewCatalogService(
	catalogs []domain.CatalogSettings,
	schemaOverrides domain.SchemaOverrides,
	pkgRepo ports.PackageRepository,
) *Catalog {
	return &Catalog{
		catalogs:       catalogs,
		pkgRepo:        pkgRepo,
		schemaResolver: newSchemaResolver(schemaOverrides),
	}
}

func (uc *Catalog) ListCatalogs(ctx context.Context, user *usercontext.User) ([]domain.Catalog, error) {
	return uc.buildCatalogs(ctx, func(c domain.CatalogSettings) bool {
		return isAccessible(user, c)
	})
}

// isAccessible reports whether user (nil when anonymous) may see or use the given
// catalog. A catalog with no restrictions is public. A restricted catalog
// requires at least one restriction to match a user attribute.
//
// This must be applied on every path that reaches a catalog or its packages
// directly (GetPackage, GetAvailableVersions, GetPackageSchema, install), not
// just on the listing endpoints — otherwise restrictions only hide a
// catalog from the UI without actually protecting it.
func isAccessible(user *usercontext.User, c domain.CatalogSettings) bool {
	if len(c.Restrictions) == 0 {
		return true
	}

	if user == nil || user.Attributes == nil {
		return false
	}
	attrs := user.Attributes

	for _, r := range c.Restrictions {
		if r.UserAttributeKey == "" || r.Match == nil {
			continue
		}

		val, ok := attrs[r.UserAttributeKey]
		if !ok {
			continue
		}

		re := r.Match

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
}

func (uc *Catalog) GetPackageSchema(
	ctx context.Context,
	user *usercontext.User,
	catalogID, packageName, version string,
) ([]byte, error) {
	cfg, err := uc.findAccessibleCatalog(user, catalogID)
	if err != nil {
		return nil, err
	}
	if slices.Contains(cfg.Excluded, packageName) {
		return nil, fmt.Errorf("%w: package %q in catalog %q", domain.ErrNotFound, packageName, catalogID)
	}

	raw, err := uc.pkgRepo.GetPackageSchema(ctx, catalogID, packageName, version)
	if err != nil {
		return nil, err
	}

	var roles []string
	if user != nil {
		roles = user.Roles
	}
	return applyOverwrites(raw, uc.schemaResolver, roles)
}

func (uc *Catalog) findCatalog(catalogID string) (*domain.CatalogSettings, error) {
	for i := range uc.catalogs {
		if uc.catalogs[i].ID == catalogID {
			return &uc.catalogs[i], nil
		}
	}
	return nil, fmt.Errorf("catalog %q: %w", catalogID, domain.ErrNotFound)
}

// findAccessibleCatalog resolves catalogID and enforces its restrictions.
// A restricted catalog the caller doesn't satisfy is reported as ErrNotFound,
// the same way it is simply omitted from ListCatalogs — this must be used
// on every direct-access path (get package, versions, schema, install), not
// just on listing, otherwise restrictions are cosmetic.
func (uc *Catalog) findAccessibleCatalog(user *usercontext.User, catalogID string) (*domain.CatalogSettings, error) {
	cfg, err := uc.findCatalog(catalogID)
	if err != nil {
		return nil, err
	}
	if !isAccessible(user, *cfg) {
		return nil, fmt.Errorf("catalog %q: %w", catalogID, domain.ErrNotFound)
	}
	return cfg, nil
}

func (uc *Catalog) GetPackage(
	ctx context.Context,
	user *usercontext.User,
	catalogID, packageName string,
) (domain.Package, error) {
	cfg, err := uc.findAccessibleCatalog(user, catalogID)
	if err != nil {
		return domain.Package{}, err
	}
	if slices.Contains(cfg.Excluded, packageName) {
		return domain.Package{}, fmt.Errorf("%w: package %q in catalog %q", domain.ErrNotFound, packageName, catalogID)
	}

	pkg, err := uc.pkgRepo.GetPackage(ctx, catalogID, packageName)
	if err != nil {
		return domain.Package{}, fmt.Errorf("catalog %q package %q: %w", catalogID, packageName, err)
	}

	return pkg, nil
}

func (uc *Catalog) GetAvailableVersions(
	ctx context.Context,
	user *usercontext.User,
	catalogID, packageName string,
) ([]string, error) {
	cfg, err := uc.findAccessibleCatalog(user, catalogID)
	if err != nil {
		return nil, err
	}
	if slices.Contains(cfg.Excluded, packageName) {
		return nil, fmt.Errorf("%w: package %q in catalog %q", domain.ErrNotFound, packageName, catalogID)
	}

	versions, err := uc.pkgRepo.GetAvailableVersions(ctx, catalogID, packageName)
	if err != nil {
		return nil, fmt.Errorf("catalog %q package %q versions: %w", catalogID, packageName, err)
	}

	filter, err := versionFilterFrom(cfg.ID, cfg.Versions)
	if err != nil {
		return nil, err
	}
	return filter.apply(versions), nil
}

// CheckSharingAllowed returns ErrForbidden if catalogID does not allow
// installed services to be shared. Restricted catalogs the caller cannot
// access are reported as ErrNotFound, same as GetPackage.
func (uc *Catalog) CheckSharingAllowed(_ context.Context, user *usercontext.User, catalogID string) error {
	cfg, err := uc.findAccessibleCatalog(user, catalogID)
	if err != nil {
		return err
	}
	if !cfg.AllowSharing {
		return fmt.Errorf("%w: catalog %q does not allow sharing", domain.ErrForbidden, catalogID)
	}
	return nil
}

func (uc *Catalog) buildCatalogs(
	ctx context.Context,
	include func(domain.CatalogSettings) bool,
) ([]domain.Catalog, error) {
	out := make([]domain.Catalog, 0)

	for _, cfg := range uc.catalogs {
		if !include(cfg) {
			continue
		}

		allPkgs, err := uc.pkgRepo.ListPackages(ctx, cfg.ID)
		if err != nil {
			return nil, fmt.Errorf("catalog %q: list packages: %w", cfg.ID, err)
		}
		pkgs := make([]domain.Package, 0, len(allPkgs))
		for _, p := range allPkgs {
			if !slices.Contains(cfg.Excluded, p.Name) {
				pkgs = append(pkgs, p)
			}
		}

		out = append(out, domain.Catalog{
			ID:                  cfg.ID,
			Name:                cfg.Name,
			Description:         cfg.Description,
			Status:              cfg.Status,
			HighlightedPackages: append([]string(nil), cfg.Highlighted...),
			Packages:            pkgs,
		})
	}

	return out, nil
}
