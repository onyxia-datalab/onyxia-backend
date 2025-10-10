package domain

import (
	"context"
)

type Catalog struct {
	ID                  string
	Name                map[string]string
	Description         map[string]string
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
