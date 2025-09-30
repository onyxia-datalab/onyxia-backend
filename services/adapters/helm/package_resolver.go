package helm

import (
	"context"
	"fmt"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

type HelmPackageResolver struct {
	catalogs []domain.Catalog
}

var _ ports.PackageResolver = (*HelmPackageResolver)(nil)

func (r *HelmPackageResolver) ResolvePackage(
	ctx context.Context,
	catalogID, pkgName, version string,
) (domain.PackageVersion, error) {

	cat, ok := r.findCatalog(catalogID)
	if !ok {
		return domain.PackageVersion{}, fmt.Errorf("catalog %q not found", catalogID)
	}

	switch cat.Type {
	case domain.CatalogTypeOCI:
		return resolveOCI(cat, pkgName, version)
	case domain.CatalogTypeHelm:
		return resolveHelm(cat, pkgName, version), nil
	default:
		return domain.PackageVersion{}, fmt.Errorf(
			"catalog %q has unsupported type %q",
			catalogID,
			cat.Type,
		)
	}
}

func (r *HelmPackageResolver) findCatalog(catalogID string) (domain.Catalog, bool) {
	for _, cat := range r.catalogs {
		if cat.ID == catalogID {
			return cat, true
		}
	}
	return domain.Catalog{}, false
}

func resolveOCI(cat domain.Catalog, pkgName, version string) (domain.PackageVersion, error) {
	for _, p := range cat.Packages {
		if p.Name != pkgName {
			continue
		}
		for _, v := range p.Versions {
			if v == version {
				return domain.PackageVersion{
					Package: domain.Package{
						Name:      pkgName,
						CatalogID: cat.ID,
					},
					Version: version,
				}, nil
			}
		}
		return domain.PackageVersion{}, fmt.Errorf(
			"version %q not found for package %q in catalog %q",
			version, pkgName, cat.ID,
		)
	}
	return domain.PackageVersion{}, fmt.Errorf(
		"package %q not found in OCI catalog %q",
		pkgName, cat.ID,
	)
}

func resolveHelm(cat domain.Catalog, pkgName, version string) domain.PackageVersion {
	return domain.PackageVersion{
		Package: domain.Package{
			Name:      pkgName,
			CatalogID: cat.ID,
		},
		Version: version,
		RepoURL: cat.URL,
	}
}
