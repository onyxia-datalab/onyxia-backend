package helm

import (
	"context"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/stretchr/testify/require"
)

func TestResolvePackage(t *testing.T) {
	ctx := context.Background()

	resolver := &HelmPackageResolver{
		catalogsConfig: []env.CatalogConfig{
			{
				ID:       "oci-cat",
				Type:     env.CatalogTypeOCI,
				Location: "https://oci.example.com",
				Packages: []env.OCIPackage{
					{
						Name:     "mypkg",
						Versions: []string{"1.0.0", "2.0.0"},
					},
				},
			},
			{
				ID:       "helm-cat",
				Type:     env.CatalogTypeHelm,
				Location: "https://charts.example.com/",
			},
			{
				ID:   "weird-cat",
				Type: "unsupported",
			},
		},
	}

	t.Run("catalog not found", func(t *testing.T) {
		_, err := resolver.ResolvePackage(ctx, "missing", "mypkg", "1.0.0")
		require.Error(t, err)
		require.Contains(t, err.Error(), `catalog "missing" not found`)
	})

	t.Run("unsupported catalog type", func(t *testing.T) {
		_, err := resolver.ResolvePackage(ctx, "weird-cat", "whatever", "1.0.0")
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported type")
	})

	t.Run("OCI package not found", func(t *testing.T) {
		_, err := resolver.ResolvePackage(ctx, "oci-cat", "unknown", "1.0.0")
		require.Error(t, err)
		require.Contains(t, err.Error(), `package "unknown" not found`)
	})

	t.Run("OCI version not found", func(t *testing.T) {
		_, err := resolver.ResolvePackage(ctx, "oci-cat", "mypkg", "9.9.9")
		require.Error(t, err)
		require.Contains(t, err.Error(), `version "9.9.9" not found`)
	})

	t.Run("OCI package and version found", func(t *testing.T) {
		pkg, err := resolver.ResolvePackage(ctx, "oci-cat", "mypkg", "2.0.0")
		require.NoError(t, err)
		require.Equal(t, "mypkg", pkg.Name)
		require.Equal(t, "2.0.0", pkg.Version)
		require.Equal(t, "oci-cat", pkg.CatalogID)
		require.Equal(
			t,
			"https://oci.example.com/mypkg",
			pkg.ChartRef(),
		)
	})

	t.Run("Helm catalog returns package and sets RepoURL", func(t *testing.T) {
		pkg, err := resolver.ResolvePackage(ctx, "helm-cat", "somepkg", "123")
		require.NoError(t, err)
		require.Equal(t, "somepkg", pkg.Name)
		require.Equal(t, "123", pkg.Version)
		require.Equal(t, "helm-cat", pkg.CatalogID)
		require.Equal(t, "https://charts.example.com/", pkg.RepoURL)
		require.Equal(t, "https://charts.example.com/somepkg", pkg.ChartRef())
	})
}

func TestChartRefHelpers(t *testing.T) {
	t.Run("PackageVersion.ChartRef trims trailing slash", func(t *testing.T) {
		pv := domain.PackageVersion{
			Package: domain.Package{
				CatalogID: "cat1",
				Name:      "mypkg",
			},
			RepoURL: "https://charts.example.com/",
			Version: "1.0.0",
		}
		require.Equal(t, "https://charts.example.com/mypkg", pv.ChartRef())
	})
}
