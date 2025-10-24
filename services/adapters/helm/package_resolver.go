package helm

import (
	"context"
	"fmt"

	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

type HelmPackageResolver struct {
	catalogsConfig []env.CatalogConfig
}

var _ ports.PackageResolver = (*HelmPackageResolver)(nil)

func NewPackageResolver(catalogs []env.CatalogConfig) ports.PackageResolver {
	copyCatalogs := append([]env.CatalogConfig(nil), catalogs...)
	return &HelmPackageResolver{catalogsConfig: copyCatalogs}
}

func (r *HelmPackageResolver) ResolvePackage(
	ctx context.Context,
	catalogID, pkgName, version string,
) (domain.PackageVersion, error) {

	cat, ok := r.findCatalog(catalogID)
	if !ok {
		return domain.PackageVersion{}, fmt.Errorf("catalog %q not found", catalogID)
	}

	switch cat.Type {
	case env.CatalogTypeOCI:
		return resolveOCI(cat, pkgName, version)
	case env.CatalogTypeHelm:
		return resolveHelm(cat, pkgName, version), nil
	default:
		return domain.PackageVersion{}, fmt.Errorf(
			"catalog %q has unsupported type %q",
			catalogID,
			cat.Type,
		)
	}
}

func (r *HelmPackageResolver) findCatalog(catalogID string) (env.CatalogConfig, bool) {
	for _, cat := range r.catalogsConfig {
		if cat.ID == catalogID {
			return cat, true
		}
	}
	return env.CatalogConfig{}, false
}

func resolveOCI(cat env.CatalogConfig, pkgName, version string) (domain.PackageVersion, error) {
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
					RepoURL: cat.Location,
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

func resolveHelm(cat env.CatalogConfig, pkgName, version string) domain.PackageVersion {
	return domain.PackageVersion{
		Package: domain.Package{
			Name:      pkgName,
			CatalogID: cat.ID,
		},
		Version: version,
		RepoURL: cat.Location,
	}
}
