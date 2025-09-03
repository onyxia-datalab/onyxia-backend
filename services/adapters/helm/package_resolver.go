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
) (domain.PackageRef, error) {

	for _, cat := range r.catalogs {
		if cat.ID != catalogID {
			continue
		}

		switch cat.Type {
		case domain.CatalogTypeOCI:
			for _, p := range cat.Packages {
				if p.PackageName != pkgName {
					continue
				}
				for _, v := range p.Versions {
					if v == version {
						return domain.PackageRef{
							PackageName: pkgName,
							Versions:    []string{version},
						}, nil
					}
				}
				return domain.PackageRef{}, fmt.Errorf(
					"version %q not found for package %q in catalog %q",
					version, pkgName, catalogID,
				)
			}
			return domain.PackageRef{}, fmt.Errorf(
				"package %q not found in OCI catalog %q",
				pkgName, catalogID,
			)

		case domain.CatalogTypeHelm:
			// Version existence is not validated here
			return domain.PackageRef{
				PackageName: pkgName,
				Versions:    []string{version},
				RepoURL:     cat.RepoURL,
			}, nil

		default:
			return domain.PackageRef{}, fmt.Errorf(
				"catalog %q has unsupported type %q",
				catalogID, cat.Type,
			)
		}
	}

	return domain.PackageRef{}, fmt.Errorf(
		"catalog %q not found",
		catalogID,
	)
}
