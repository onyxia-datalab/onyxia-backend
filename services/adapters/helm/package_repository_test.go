package helm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/repo"
)

type localHelmRepo struct {
	server   *httptest.Server
	tmpDir   string
	settings *cli.EnvSettings
	cfg      env.CatalogConfig
}

func newLocalHelmRepo(t *testing.T, charts ...*chart.Metadata) *localHelmRepo {
	t.Helper()

	tmp := t.TempDir()
	settings := cli.New()
	settings.RepositoryCache = tmp

	// Crée un index.yaml local
	idx := repo.NewIndexFile()
	for _, c := range charts {
		if err := idx.MustAdd(c, c.Name+"-"+c.Version+".tgz", "http://localhost", ""); err != nil {
			t.Fatalf("failed to add chart %s: %v", c.Name, err)
		}
	}
	indexPath := filepath.Join(tmp, "index.yaml")
	require.NoError(t, idx.WriteFile(indexPath, 0644))

	// Serveur HTTP local
	mux := http.NewServeMux()
	mux.Handle("/index.yaml", http.FileServer(http.Dir(tmp)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cfg := env.CatalogConfig{
		ID:       "test",
		Type:     env.CatalogTypeHelm,
		Location: server.URL,
	}

	return &localHelmRepo{
		server:   server,
		tmpDir:   tmp,
		settings: settings,
		cfg:      cfg,
	}
}

func (l *localHelmRepo) newAdapter(t *testing.T) *HelmPackageRepository {
	t.Helper()
	repoAdapter, err := NewPackageRepository(
		[]env.CatalogConfig{l.cfg},
		WithHelmSettings(l.settings),
	)
	require.NoError(t, err)
	return repoAdapter
}

func TestListHelmPackages_WithLocalHTTPRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	lr := newLocalHelmRepo(t, &chart.Metadata{
		Name:        "mychart",
		Version:     "1.0.0",
		Description: "Fake chart for testing",
		Home:        "https://onyxia.sh",
		Icon:        "https://onyxia.sh/icon.png",
	})

	repoAdapter := lr.newAdapter(t)

	result, err := repoAdapter.ListPackages(context.Background(), lr.cfg)
	require.NoError(t, err)
	require.Len(t, result, 1)

	pkg := result[0]
	require.Equal(t, "mychart", pkg.Name)
	require.Equal(t, "Fake chart for testing", pkg.Description)
	require.Equal(t, "https://onyxia.sh/icon.png", pkg.IconUrl.String())
	require.Equal(t, "https://onyxia.sh", pkg.HomeUrl.String())
	require.Equal(t, lr.cfg.ID, pkg.CatalogID)

	require.NoError(t, os.RemoveAll(lr.tmpDir))
}

func TestResolvePackage_HelmRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	lr := newLocalHelmRepo(t, &chart.Metadata{
		Name:        "mychart",
		Version:     "1.0.0",
		Description: "fake chart",
	})

	repoAdapter := lr.newAdapter(t)

	t.Run("existing chart and version", func(t *testing.T) {
		pkg, err := repoAdapter.ResolvePackage(context.Background(), lr.cfg.ID, "mychart", "1.0.0")
		require.NoError(t, err)
		require.Equal(t, "mychart", pkg.Name)
		require.Equal(t, "1.0.0", pkg.Version)
	})

	t.Run("chart not found", func(t *testing.T) {
		_, err := repoAdapter.ResolvePackage(context.Background(), lr.cfg.ID, "unknown", "1.0.0")
		require.Error(t, err)
		require.Contains(t, err.Error(), `chart "unknown" not found`)
	})

	t.Run("version not found", func(t *testing.T) {
		_, err := repoAdapter.ResolvePackage(context.Background(), lr.cfg.ID, "mychart", "9.9.9")
		require.Error(t, err)
		require.Contains(t, err.Error(), `version "9.9.9" not found`)
	})
}
