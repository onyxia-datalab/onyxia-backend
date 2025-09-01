package domain

import "fmt"

type PackageKind string

const (
	PackageKindHelm PackageKind = "helm"
	PackageKindOCI  PackageKind = "oci"
)

type PackageRef struct {
	Kind      PackageKind
	RepoURL   string // ex: "ghcr.io/onyxia-datalab/charts" or "bitnamilegacy/postgresql"
	ChartName string // ex: "jupyter-python"
	Version   string // ex: "1.2.3"
}

func (r PackageRef) ChartRef() string {
	switch r.Kind {
	case PackageKindHelm:
		return fmt.Sprintf("%s/%s", r.RepoURL, r.ChartName)
	case PackageKindOCI:
		return fmt.Sprintf("oci://%s/%s", r.RepoURL, r.ChartName)
	default:
		panic(fmt.Sprintf("unsupported package kind: %s", r.Kind))
	}
}
