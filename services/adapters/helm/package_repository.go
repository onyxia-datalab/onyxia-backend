package helm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/onyxia-datalab/onyxia-backend/internal/tools"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/repo/v1"
)

var _ ports.PackageRepository = (*HelmPackageRepository)(nil)

// ensure chart package is referenced (used via chart.Charter interface from loader)
var _ chart.Charter = (*chartv2.Chart)(nil)

type HelmPackageRepository struct {
	repos    map[string]*repo.ChartRepository
	catalogs map[string]env.CatalogConfig
	getters  getter.Providers
	settings *cli.EnvSettings
}

// --- Public (port interface) ---

func NewPackageRepository(
	catalogs []env.CatalogConfig,
	cacheDir string,
) (*HelmPackageRepository, error) {
	settings := cli.New()
	if cacheDir != "" {
		settings.RepositoryCache = cacheDir
	}

	repos := make(map[string]*repo.ChartRepository)
	catalogMap := make(map[string]env.CatalogConfig)
	getters := getter.All(settings)

	for _, cfg := range catalogs {
		catalogMap[cfg.ID] = cfg

		if _, err := versionFilterFrom(cfg); err != nil {
			return nil, err
		}

		if cfg.Type != env.CatalogTypeHelmRepo {
			continue
		}

		entry := &repo.Entry{
			Name:                  cfg.ID,
			URL:                   cfg.Location,
			Username:              tools.Deref(cfg.Username),
			Password:              tools.Deref(cfg.Password),
			InsecureSkipTLSVerify: cfg.SkipTLSVerify,
			CAFile:                tools.Deref(cfg.CAFile),
		}

		cr, err := repo.NewChartRepository(entry, getters)
		if err != nil {
			return nil, fmt.Errorf("failed to create repo %q: %w", cfg.ID, err)
		}
		cr.CachePath = settings.RepositoryCache
		repos[cfg.ID] = cr

		slog.Info(
			"Helm repo configured",
			slog.String("catalog", cfg.ID),
			slog.String("url", cfg.Location),
			slog.String("cache", settings.RepositoryCache),
		)
	}

	return &HelmPackageRepository{
		repos:    repos,
		catalogs: catalogMap,
		getters:  getters,
		settings: settings,
	}, nil
}

func (h *HelmPackageRepository) ListPackages(
	ctx context.Context,
	catalogID string,
) ([]domain.Package, error) {
	cfg, ok := h.catalogs[catalogID]
	if !ok {
		return nil, fmt.Errorf("%w: catalog %q not found", domain.ErrNotFound, catalogID)
	}

	switch cfg.Type {
	case env.CatalogTypeHelmRepo:
		cvs, err := h.listCharts(catalogID)
		if err != nil {
			return nil, err
		}
		pkgs := make([]domain.Package, 0, len(cvs))
		for _, cv := range cvs {
			pkgs = append(pkgs, domain.Package{
				CatalogID:   cfg.ID,
				Name:        cv.Name,
				Description: cv.Description,
				HomeUrl:     tools.MustParseURL(cv.Home),
				IconUrl:     tools.MustParseURL(cv.Icon),
			})
		}
		return pkgs, nil
	case env.CatalogTypeOCI:
		return h.listOCIPackages(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported catalog type: %v", cfg.Type)
	}
}

func (h *HelmPackageRepository) GetPackage(
	ctx context.Context,
	catalogID string,
	name string,
) (domain.Package, error) {
	cfg, ok := h.catalogs[catalogID]
	if !ok {
		return domain.Package{}, fmt.Errorf(
			"%w: catalog %q not found",
			domain.ErrNotFound,
			catalogID,
		)
	}

	switch cfg.Type {
	case env.CatalogTypeHelmRepo:
		cv, err := h.getChart(catalogID, name)
		if err != nil {
			return domain.Package{}, err
		}
		return domain.Package{
			CatalogID:   cfg.ID,
			Name:        cv.Name,
			Description: cv.Description,
			HomeUrl:     tools.MustParseURL(cv.Home),
			IconUrl:     tools.MustParseURL(cv.Icon),
		}, nil
	case env.CatalogTypeOCI:
		return h.getOCIPackage(cfg, name)
	default:
		return domain.Package{}, fmt.Errorf("unsupported catalog type: %v", cfg.Type)
	}
}

func (h *HelmPackageRepository) GetAvailableVersions(
	ctx context.Context,
	catalogID string,
	name string,
) ([]string, error) {
	cfg, ok := h.catalogs[catalogID]
	if !ok {
		return nil, fmt.Errorf("%w: catalog %q not found", domain.ErrNotFound, catalogID)
	}

	switch cfg.Type {
	case env.CatalogTypeHelmRepo:
		cvs, err := h.getChartVersions(catalogID, name)
		if err != nil {
			return nil, err
		}
		var versions []string
		for _, cv := range cvs {
			versions = append(versions, cv.Version)
		}
		return versions, nil
	case env.CatalogTypeOCI:
		p, err := findOCIPackage(cfg, name)
		if err != nil {
			return nil, err
		}

		return p.Versions, nil
	default:
		return nil, fmt.Errorf("unsupported catalog type: %v", cfg.Type)
	}
}

func (h *HelmPackageRepository) GetPackageSchema(
	ctx context.Context,
	catalogID string,
	packageName string,
	version string,
) ([]byte, error) {
	cfg, ok := h.catalogs[catalogID]
	if !ok {
		return nil, fmt.Errorf("%w: catalog %q not found", domain.ErrNotFound, catalogID)
	}

	chartRef := fmt.Sprintf("%s/%s", strings.TrimSuffix(cfg.Location, "/"), packageName)

	act := action.NewInstall(new(action.Configuration))
	act.Version = version

	chartPath, err := act.LocateChart(chartRef, h.settings)
	if err != nil {
		return nil, fmt.Errorf("locating chart %q: %w", chartRef, err)
	}

	raw, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("loading chart: %w", err)
	}
	ch, ok2 := raw.(*chartv2.Chart)
	if !ok2 {
		return nil, fmt.Errorf("unexpected chart type %T", raw)
	}
	return ch.Schema, nil
}

// --- helm repo (index) ---

// TODO: we could optimize by caching the index in memory and refreshing it with a TTL (same as helm index cache)
func (h *HelmPackageRepository) loadIndex(catalogName string) (*repo.IndexFile, error) {
	cr := h.repos[catalogName]
	indexPath, err := cr.DownloadIndexFile()
	if err != nil {
		return nil, fmt.Errorf("downloading helm index for %q: %w", catalogName, err)
	}
	idx, err := repo.LoadIndexFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("loading helm index for %q: %w", catalogName, err)
	}
	return idx, nil
}

// listCharts returns the latest version of each chart (helm search repo <catalog>).
func (h *HelmPackageRepository) listCharts(catalogName string) (repo.ChartVersions, error) {
	idx, err := h.loadIndex(catalogName)
	if err != nil {
		return nil, err
	}

	var result repo.ChartVersions
	var errs []error
	for name := range idx.Entries {
		cv, err := idx.Get(name, "")
		if err != nil {
			errs = append(errs, fmt.Errorf("getting chart version for %q: %w", name, err))
			continue
		}
		result = append(result, cv)
	}
	return result, errors.Join(errs...)
}

// getChart returns the latest version of a single chart (helm search repo <catalog>/<package>).
func (h *HelmPackageRepository) getChart(
	catalogName, packageName string,
) (*repo.ChartVersion, error) {
	idx, err := h.loadIndex(catalogName)
	if err != nil {
		return nil, err
	}

	cv, err := idx.Get(packageName, "")
	if err != nil {
		return nil, fmt.Errorf("getting chart %q: %w", packageName, err)
	}
	if cv == nil {
		return nil, fmt.Errorf(
			"%w: chart %q not found in repo %q",
			domain.ErrNotFound,
			packageName,
			catalogName,
		)
	}

	return cv, nil
}

// getChartVersions returns all versions of a chart (helm search repo <catalog>/<package> --versions).
func (h *HelmPackageRepository) getChartVersions(
	catalogName, packageName string,
) (repo.ChartVersions, error) {
	idx, err := h.loadIndex(catalogName)
	if err != nil {
		return nil, err
	}
	versions, ok := idx.Entries[packageName]
	if !ok {
		return nil, fmt.Errorf(
			"%w: chart %q not found in repo %q",
			domain.ErrNotFound,
			packageName,
			catalogName,
		)
	}
	return versions, nil
}

// --- OCI ---

func (h *HelmPackageRepository) listOCIPackages(cfg env.CatalogConfig) []domain.Package {
	pkgs := make([]domain.Package, 0, len(cfg.Packages))
	for _, p := range cfg.Packages {
		pkgs = append(pkgs, domain.Package{
			CatalogID: cfg.ID,
			Name:      p.Name,
		})
	}
	return pkgs
}

func (h *HelmPackageRepository) getOCIPackage(
	cfg env.CatalogConfig,
	name string,
) (domain.Package, error) {
	p, err := findOCIPackage(cfg, name)
	if err != nil {
		return domain.Package{}, err
	}
	// TODO: pull chart to get metadata (description, icon, home URL)
	return domain.Package{
		CatalogID: cfg.ID,
		Name:      p.Name,
	}, nil
}

func findOCIPackage(cfg env.CatalogConfig, name string) (*env.OCIPackage, error) {
	for i := range cfg.Packages {
		if cfg.Packages[i].Name == name {
			return &cfg.Packages[i], nil
		}
	}
	return nil, fmt.Errorf(
		"%w: package %q not found in OCI catalog %q",
		domain.ErrNotFound,
		name,
		cfg.ID,
	)
}
