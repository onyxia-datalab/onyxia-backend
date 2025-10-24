package helm

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/onyxia-datalab/onyxia-backend/internal/tools"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

var _ ports.CatalogRepository = (*HelmCatalogRepository)(nil)

type HelmCatalogRepository struct {
	settings *cli.EnvSettings
	repos    map[string]*repo.ChartRepository
}

func NewCatalogRepo(
	settings *cli.EnvSettings,
	cfgs []env.CatalogConfig,
) (*HelmCatalogRepository, error) {
	var repos = make(map[string]*repo.ChartRepository)

	//TODO There is issue catalog contain multiple packages
	for _, cfg := range cfgs {
		entry := &repo.Entry{
			Name:                  cfg.ID,
			URL:                   cfg.Location,
			Username:              tools.Deref(cfg.Username),
			Password:              tools.Deref(cfg.Password),
			InsecureSkipTLSverify: cfg.SkipTLSVerify,
			CAFile:                tools.Deref(cfg.CAFile),
		}

		chartRepo, err := repo.NewChartRepository(entry, getter.All(settings))
		if err != nil {
			return nil, fmt.Errorf("failed to create repo %q: %w", cfg.ID, err)
		}

		repos[cfg.ID] = chartRepo
	}

	return &HelmCatalogRepository{
		settings: settings,
		repos:    repos,
	}, nil
}

func (h *HelmCatalogRepository) ListPackages(
	ctx context.Context,
	cfg env.CatalogConfig,
) ([]domain.PackageRef, error) {

	chartRepo, ok := h.repos[cfg.ID]
	if !ok {
		return nil, fmt.Errorf("unknown helm catalog %q", cfg.ID)
	}

	indexFile := filepath.Join(h.settings.RepositoryCache, fmt.Sprintf("%s-index.yaml", cfg.ID))
	if _, err := chartRepo.DownloadIndexFile(); err != nil {
		return nil, fmt.Errorf("fetching helm repo index: %w", err)
	}

	index, err := repo.LoadIndexFile(indexFile)
	if err != nil {
		return nil, fmt.Errorf("parsing helm repo index: %w", err)
	}

	pkgs := make([]domain.PackageRef, 0, len(index.Entries))
	for chartName, versions := range index.Entries {
		if len(versions) == 0 {
			continue
		}

		latest := versions[0]
		pkgs = append(pkgs, domain.PackageRef{
			Package: domain.Package{
				CatalogID:   cfg.ID,
				Name:        chartName,
				Description: latest.Description,
				HomeUrl:     mustParseURL(latest.Home),
				IconUrl:     mustParseURL(latest.Icon),
			},
			Versions: extractVersions(versions),
		})
	}
	return pkgs, nil
}

func extractVersions(charts []*repo.ChartVersion) []string {
	versions := make([]string, 0, len(charts))
	for _, v := range charts {
		versions = append(versions, v.Version)
	}
	return versions
}

func mustParseURL(s string) url.URL {
	u, _ := url.Parse(s)
	if u == nil {
		return url.URL{}
	}
	return *u
}
