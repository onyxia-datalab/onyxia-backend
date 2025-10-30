package helm_test

import (
	"context"
	"path/filepath"
	"testing"

	"net/http"
	"net/http/httptest"
	"os"

	"github.com/onyxia-datalab/onyxia-backend/services/adapters/helm"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/repo"
)

func TestListHelmPackages_WithLocalHTTPRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmp := t.TempDir()
	t.Cleanup(func() {
		if err := os.RemoveAll(tmp); err != nil {
			t.Logf("cleanup failed: %v", err)
		}
	})

	settings := cli.New()
	settings.RepositoryCache = tmp

	mux := http.NewServeMux()
	mux.Handle("/index.yaml", http.FileServer(http.Dir(tmp)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	idx := repo.NewIndexFile()
	
	if err := idx.MustAdd(&chart.Metadata{
		Name:        "mychart",
		Version:     "1.0.0",
		Description: "Fake chart for testing",
		Home:        "https://onyxia.sh",
		Icon:        "https://onyxia.sh/icon.png",
	}, "mychart-1.0.0.tgz", server.URL, ""); err != nil {
		t.Fatalf("failed to add chart to index: %v", err)
	}

	indexPath := filepath.Join(tmp, "index.yaml")
	assert.NoError(t, idx.WriteFile(indexPath, 0644))

	cfg := env.CatalogConfig{
		ID:       "test",
		Type:     env.CatalogTypeHelm,
		Location: server.URL,
	}

	repoAdapter, err := helm.NewCatalogRepo(settings, []env.CatalogConfig{cfg})
	assert.NoError(t, err)

	result, err := repoAdapter.ListPackages(context.Background(), cfg)
	assert.NoError(t, err)
	assert.Len(t, result, 1)

	pkg := result[0]
	assert.Equal(t, "mychart", pkg.Name)
	assert.Equal(t, "Fake chart for testing", pkg.Description)
	assert.Equal(t, "https://onyxia.sh/icon.png", pkg.IconUrl.String())
	assert.Equal(t, "https://onyxia.sh", pkg.HomeUrl.String())
	assert.Equal(t, cfg.ID, pkg.CatalogID)
}
