package domain

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/internal/tools"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
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
	ListUserCatalog(ctx context.Context, user usercontext.Reader,
	) ([]Catalog, error)
}
