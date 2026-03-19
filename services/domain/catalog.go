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
	GetPackage(ctx context.Context, catalogID string, packageName string) (*PackageRef, error)
	GetPackageSchema(
		ctx context.Context,
		catalogID string,
		packageName string,
		version string,
	) ([]byte, error)
}
