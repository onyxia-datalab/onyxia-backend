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
	RepoURL     string
}

func (p Package) ChartRef() string {
	if strings.HasPrefix(p.RepoURL, "oci://") {
		return p.RepoURL
	}
	return fmt.Sprintf("%s/%s", strings.TrimSuffix(p.RepoURL, "/"), p.Name)
}
