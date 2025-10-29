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

// 🧩 This test is an integration test for the HelmCatalogRepository adapter.
// It uses an in-memory HTTP server (httptest.Server) to simulate a Helm repository.
// No external network calls are made.
func TestListHelmPackages_WithLocalHTTPRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmp := t.TempDir()
	settings := cli.New()
	settings.RepositoryCache = tmp

	// --- 2️⃣ Serve the fake index.yaml through a local in-memory HTTP server ---
	mux := http.NewServeMux()
	mux.Handle("/index.yaml", http.FileServer(http.Dir(tmp)))
	server := httptest.NewServer(mux)
	defer server.Close()

	// --- 1️⃣ Create a fake Helm index.yaml using Helm’s own API ---
	idx := repo.NewIndexFile()
	idx.MustAdd(&chart.Metadata{
		Name:        "mychart",
		Version:     "1.0.0",
		Description: "Fake chart for testing",
		Home:        "https://onyxia.sh",
		Icon:        "https://onyxia.sh/icon.png",
	}, "mychart-1.0.0.tgz", server.URL, "")

	indexPath := filepath.Join(tmp, "index.yaml")
	assert.NoError(t, idx.WriteFile(indexPath, 0644))

	// --- 3️⃣ Create a catalog config pointing to the fake repo ---
	cfg := env.CatalogConfig{
		ID:       "test",
		Type:     env.CatalogTypeHelm,
		Location: server.URL, // Helm will GET /index.yaml here
	}

	repoAdapter, err := helm.NewCatalogRepo(settings, []env.CatalogConfig{cfg})
	assert.NoError(t, err, "should create HelmCatalogRepository without error")

	// --- 4️⃣ Run the method under test ---
	result, err := repoAdapter.ListPackages(context.Background(), cfg)

	// --- 5️⃣ Validate results ---
	assert.NoError(t, err)
	assert.Len(t, result, 1, "should list one chart")
	pkg := result[0]
	assert.Equal(t, "mychart", pkg.Name)
	assert.Equal(t, "Fake chart for testing", pkg.Description)
	assert.Equal(t, "https://onyxia.sh/icon.png", pkg.IconUrl.String())
	assert.Equal(t, "https://onyxia.sh", pkg.HomeUrl.String())
	assert.Equal(t, cfg.ID, pkg.CatalogID)

	// --- Cleanup ---
	os.RemoveAll(tmp)
}
