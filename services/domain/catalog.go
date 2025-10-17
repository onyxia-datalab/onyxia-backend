package domain

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/internal/utils"
)

type Catalog struct {
	ID                  string
	Name                utils.LocalizedString
	Description         utils.LocalizedString
	Status              CatalogStatus
	HighlightedPackages []string
	Visible             CatalogVisibility
	Packages            []Package
}

type CatalogStatus string

const (
	CatalogStatusProd CatalogStatus = "PROD"
	CatalogStatusTest CatalogStatus = "TEST"
)

type CatalogVisibility struct {
	User    bool
	Project bool
}

type CatalogService interface {
	ListPublicCatalogs(ctx context.Context) ([]Catalog, error)
	ListUserCatalog(ctx context.Context) ([]Catalog, error)
}
