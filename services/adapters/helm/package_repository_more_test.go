package helm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	chartutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/repo/v1"
)

// newChartRepoServer serves packaged charts and their index, so that charts
// can be downloaded, not only listed.
func newChartRepoServer(t *testing.T, charts ...*chartv2.Chart) env.CatalogConfig {
	t.Helper()

	dir := t.TempDir()
	server := httptest.NewServer(http.FileServer(http.Dir(dir)))
	t.Cleanup(server.Close)

	for _, c := range charts {
		_, err := chartutil.Save(c, dir)
		require.NoError(t, err)
	}
	idx, err := repo.IndexDirectory(dir, server.URL)
	require.NoError(t, err)
	require.NoError(t, idx.WriteFile(filepath.Join(dir, "index.yaml"), 0o644))

	return env.CatalogConfig{ID: "test", Type: env.CatalogTypeHelmRepo, Location: server.URL}
}

func newAdapterFor(t *testing.T, catalogs ...env.CatalogConfig) *HelmPackageRepository {
	t.Helper()
	// Keep the developer's own Helm repositories out of chart downloads.
	t.Setenv("HELM_REPOSITORY_CONFIG", filepath.Join(t.TempDir(), "repositories.yaml"))
	client, err := NewClient(t.TempDir())
	require.NoError(t, err)
	r, err := NewPackageRepository(catalogs, client)
	require.NoError(t, err)
	return r
}

func ociCatalog() env.CatalogConfig {
	return env.CatalogConfig{
		ID:   "oci",
		Type: env.CatalogTypeOCI,
		Packages: []env.OCIPackage{
			{Name: "jupyter", ChartRef: "oci://registry.example.com/charts/jupyter"},
			{Name: "vscode", ChartRef: "oci://127.0.0.1:1/charts/vscode", Versions: []string{"1.2.0", "1.1.0"}},
		},
	}
}

func TestPackageRepository_UnknownCatalog(t *testing.T) {
	r := newAdapterFor(t)
	ctx := context.Background()

	_, err := r.ListPackages(ctx, "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = r.GetPackage(ctx, "missing", "jupyter")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = r.GetAvailableVersions(ctx, "missing", "jupyter")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = r.GetPackageSchema(ctx, "missing", "jupyter", "1.0.0")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPackageRepository_UnsupportedCatalogType(t *testing.T) {
	r := newAdapterFor(t, env.CatalogConfig{ID: "git", Type: "git"})
	ctx := context.Background()

	_, err := r.ListPackages(ctx, "git")
	assert.ErrorContains(t, err, "unsupported catalog type")
	_, err = r.GetPackage(ctx, "git", "jupyter")
	assert.ErrorContains(t, err, "unsupported catalog type")
	_, err = r.GetAvailableVersions(ctx, "git", "jupyter")
	assert.ErrorContains(t, err, "unsupported catalog type")
}

func TestOCIPackages(t *testing.T) {
	r := newAdapterFor(t, ociCatalog())
	ctx := context.Background()

	pkgs, err := r.ListPackages(ctx, "oci")
	require.NoError(t, err)
	assert.Equal(t, []domain.Package{
		{CatalogID: "oci", Name: "jupyter", ChartRef: "oci://registry.example.com/charts/jupyter"},
		{CatalogID: "oci", Name: "vscode", ChartRef: "oci://127.0.0.1:1/charts/vscode"},
	}, pkgs)

	// Without pinned versions, the package is described from the config alone.
	pkg, err := r.GetPackage(ctx, "oci", "jupyter")
	require.NoError(t, err)
	assert.Equal(t, domain.Package{
		CatalogID: "oci",
		Name:      "jupyter",
		ChartRef:  "oci://registry.example.com/charts/jupyter",
	}, pkg)

	versions, err := r.GetAvailableVersions(ctx, "oci", "vscode")
	require.NoError(t, err)
	assert.Equal(t, []string{"1.2.0", "1.1.0"}, versions)
}

func TestOCIPackages_NotFound(t *testing.T) {
	r := newAdapterFor(t, ociCatalog())
	ctx := context.Background()

	_, err := r.GetPackage(ctx, "oci", "unknown")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = r.GetAvailableVersions(ctx, "oci", "unknown")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = r.GetPackageSchema(ctx, "oci", "unknown", "1.0.0")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestOCIPackage_UnreachableRegistry(t *testing.T) {
	r := newAdapterFor(t, ociCatalog())

	_, err := r.GetPackage(context.Background(), "oci", "vscode")

	assert.ErrorContains(t, err, "locating OCI chart")
}

func TestHelmRepo_UnreachableIndex(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	r := newAdapterFor(t, env.CatalogConfig{ID: "test", Type: env.CatalogTypeHelmRepo, Location: server.URL})
	ctx := context.Background()

	_, err := r.ListPackages(ctx, "test")
	assert.ErrorContains(t, err, "downloading helm index")
	_, err = r.GetPackage(ctx, "test", "jupyter")
	assert.ErrorContains(t, err, "downloading helm index")
	_, err = r.GetAvailableVersions(ctx, "test", "jupyter")
	assert.ErrorContains(t, err, "downloading helm index")
}

func TestGetAvailableVersions_UnknownChart(t *testing.T) {
	lr := newLocalHelmRepo(t, &chartv2.Metadata{Name: "mychart", Version: "1.0.0"})

	_, err := lr.newAdapter(t).GetAvailableVersions(context.Background(), lr.cfg.ID, "unknown")

	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetPackageSchema_HelmRepo(t *testing.T) {
	cfg := newChartRepoServer(t, &chartv2.Chart{
		Metadata: &chartv2.Metadata{APIVersion: chartv2.APIVersionV2, Name: "mychart", Version: "1.0.0"},
		Schema:   []byte(`{"type":"object"}`),
	})
	r := newAdapterFor(t, cfg)

	schema, err := r.GetPackageSchema(context.Background(), cfg.ID, "mychart", "1.0.0")

	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"object"}`, string(schema))
}

func TestGetPackageSchema_UnknownVersion(t *testing.T) {
	cfg := newChartRepoServer(t, &chartv2.Chart{
		Metadata: &chartv2.Metadata{APIVersion: chartv2.APIVersionV2, Name: "mychart", Version: "1.0.0"},
	})
	r := newAdapterFor(t, cfg)

	_, err := r.GetPackageSchema(context.Background(), cfg.ID, "mychart", "9.9.9")

	assert.ErrorContains(t, err, `locating chart "mychart"`)
}
