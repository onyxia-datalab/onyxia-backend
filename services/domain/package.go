package domain

import "net/url"

type Package struct {
	CatalogID   string
	Name        string
	Description string
	HomeUrl     url.URL
	IconUrl     url.URL
	RepoURL     string // helm repo URL (empty for OCI)
	ChartRef    string // full OCI reference, e.g. oci://registry/path/chart (empty for helm repos)
}
