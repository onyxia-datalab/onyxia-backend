package helm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/repo/v1"
)

type localHelmRepo struct {
	server *httptest.Server
	tmpDir string
	cfg    env.CatalogConfig
}

func newLocalHelmRepo(t *testing.T, charts ...*chartv2.Metadata) *localHelmRepo {
	t.Helper()

	tmp := t.TempDir()

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
		Type:     env.CatalogTypeHelmRepo,
		Location: server.URL,
	}

	return &localHelmRepo{
		server: server,
		tmpDir: tmp,
		cfg:    cfg,
	}
}

func (l *localHelmRepo) newAdapter(t *testing.T) *HelmPackageRepository {
	t.Helper()
	repoAdapter, err := NewPackageRepository([]env.CatalogConfig{l.cfg}, l.tmpDir)
	require.NoError(t, err)
	return repoAdapter
}

func TestListHelmPackages_WithLocalHTTPRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	lr := newLocalHelmRepo(t, &chartv2.Metadata{
		Name:        "mychart",
		Version:     "1.0.0",
		Description: "Fake chart for testing",
		Home:        "https://onyxia.sh",
		Icon:        "https://onyxia.sh/icon.png",
	})

	repoAdapter := lr.newAdapter(t)

	result, err := repoAdapter.ListPackages(context.Background(), lr.cfg.ID)
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

func TestGetHelmPackage_Found(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	lr := newLocalHelmRepo(t,
		&chartv2.Metadata{Name: "mychart", Version: "2.0.0", Description: "v2"},
		&chartv2.Metadata{Name: "mychart", Version: "1.0.0", Description: "v1"},
		&chartv2.Metadata{Name: "other", Version: "1.0.0"},
	)
	repoAdapter := lr.newAdapter(t)

	pkg, err := repoAdapter.GetPackage(context.Background(), lr.cfg.ID, "mychart")
	require.NoError(t, err)
	assert.Equal(t, "mychart", pkg.Name)
	assert.Equal(t, "v2", pkg.Description)
}

func TestGetHelmPackage_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	lr := newLocalHelmRepo(t, &chartv2.Metadata{Name: "mychart", Version: "1.0.0"})
	repoAdapter := lr.newAdapter(t)

	_, err := repoAdapter.GetPackage(context.Background(), lr.cfg.ID, "unknown")
	require.Error(t, err)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetAvailableVersions_All(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	lr := newLocalHelmRepo(t,
		&chartv2.Metadata{Name: "mychart", Version: "2.0.0"},
		&chartv2.Metadata{Name: "mychart", Version: "1.0.0"},
	)
	repoAdapter := lr.newAdapter(t)

	versions, err := repoAdapter.GetAvailableVersions(context.Background(), lr.cfg.ID, "mychart")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"2.0.0", "1.0.0"}, versions)
}
