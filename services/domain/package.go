package domain

import (
	"fmt"
	"net/url"
	"strings"
)

type Package struct {
	CatalogID   string
	Name        string
	Description string
	HomeUrl     url.URL
	IconUrl     url.URL
}

type PackageRef struct {
	Package
	Versions []string
}

type PackageVersion struct {
	Package
	Version string
	RepoURL string
}

func (r PackageVersion) ChartRef() string {
	return fmt.Sprintf("%s/%s", strings.TrimSuffix(r.RepoURL, "/"), r.Name)
}
