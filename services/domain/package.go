package domain

import (
	"fmt"
	"strings"
)

type Package struct {
	CatalogID string
	Name      string
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
