package domain

type CatalogType string

const (
	CatalogTypeHelm CatalogType = "helm"
	CatalogTypeOCI  CatalogType = "oci"
)

type Catalog struct {
	ID       string
	Type     CatalogType
	RepoURL  string
	Packages []PackageRef // used only for OCI catalogs
}
