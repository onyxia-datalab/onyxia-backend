package domain

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/internal/tools"
)

type Catalog struct {
	ID                  string
	Name                tools.LocalizedString
	Description         tools.LocalizedString
	Status              CatalogStatus
	HighlightedPackages []string
	Packages            []Package
}

type CatalogStatus string

const (
	CatalogStatusProd CatalogStatus = "PROD"
	CatalogStatusTest CatalogStatus = "TEST"
)

type CatalogService interface {
	ListPublicCatalogs(ctx context.Context) ([]Catalog, error)
	ListUserCatalogs(ctx context.Context) ([]Catalog, error)
	// GetPackage returns ErrNotFound if the package is excluded, or if the
	// catalog is restricted and the caller does not satisfy any restriction.
	GetPackage(ctx context.Context, catalogID string, packageName string) (Package, error)
	GetAvailableVersions(ctx context.Context, catalogID string, packageName string) ([]string, error)
	GetPackageSchema(
		ctx context.Context,
		catalogID string,
		packageName string,
		version string,
	) ([]byte, error)
	// CheckSharingAllowed returns ErrForbidden if the catalog does not allow
	// installed services to be shared, and ErrNotFound under the same
	// conditions as GetPackage.
	CheckSharingAllowed(ctx context.Context, catalogID string) error
}
