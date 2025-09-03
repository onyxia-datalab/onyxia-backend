package domain

import (
	"fmt"
	"strings"
)

type PackageRef struct {
	RepoURL     string   // ex: "oci://ghcr.io/onyxia-datalab/charts" or "https://inseefrlab.github.io/helm-charts-interactive-services"
	PackageName string   // ex: "jupyter-python"
	Versions    []string // ex: ["1.2.3","1.2.4"]
}

func (r PackageRef) ChartRef() string {
	return fmt.Sprintf("%s/%s", strings.TrimSuffix(r.RepoURL, "/"), r.PackageName)
}
