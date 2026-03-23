package helm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

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
	"helm.sh/helm/v4/pkg/registry"
	"helm.sh/helm/v4/pkg/repo/v1"
)

var _ ports.PackageRepository = (*HelmPackageRepository)(nil)

// ensure chart package is referenced (used via chart.Charter interface from loader)
var _ chart.Charter = (*chartv2.Chart)(nil)

// ociPackageTTL is effectively infinite: OCI versions are immutable so
// cached metadata never goes stale. Eviction happens on process restart.
const ociPackageTTL = time.Duration(1<<63 - 1)

type HelmPackageRepository struct {
	repos          map[string]*repo.ChartRepository
	catalogs       map[string]env.CatalogConfig
	getters        getter.Providers
	settings       *cli.EnvSettings
	indexes        *tools.TTLCache[string, *repo.IndexFile]
	ociPkgs        *tools.TTLCache[string, domain.Package]
	registryClient *registry.Client
}

// --- Public (port interface) ---

func NewPackageRepository(
	catalogs []env.CatalogConfig,
	client *Client,
) (*HelmPackageRepository, error) {
	repos := make(map[string]*repo.ChartRepository)
	catalogMap := make(map[string]env.CatalogConfig)

	for _, cfg := range catalogs {
		catalogMap[cfg.ID] = cfg

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

		cr, err := repo.NewChartRepository(entry, client.Getters)
		if err != nil {
			return nil, fmt.Errorf("failed to create repo %q: %w", cfg.ID, err)
		}
		cr.CachePath = client.Settings.RepositoryCache
		repos[cfg.ID] = cr

		slog.Info(
			"Helm repo configured",
			slog.String("catalog", cfg.ID),
			slog.String("url", cfg.Location),
			slog.String("cache", client.Settings.RepositoryCache),
		)
	}

	return &HelmPackageRepository{
		repos:          repos,
		catalogs:       catalogMap,
		getters:        client.Getters,
		settings:       client.Settings,
		indexes:        tools.NewTTLCache[string, *repo.IndexFile](),
		ociPkgs:        tools.NewTTLCache[string, domain.Package](),
		registryClient: client.RegistryClient,
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

	schemaCfg := &action.Configuration{RegistryClient: h.registryClient}
	act := action.NewInstall(schemaCfg)
	act.Version = version

	var chartRef string
	switch cfg.Type {
	case env.CatalogTypeHelmRepo:
		act.RepoURL = cfg.Location
		chartRef = packageName
	default: // OCI
		chartRef = fmt.Sprintf("%s/%s", strings.TrimSuffix(cfg.Location, "/"), packageName)
	}

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

func (h *HelmPackageRepository) loadIndex(catalogName string) (*repo.IndexFile, error) {
	return h.indexes.Get(
		catalogName,
		h.catalogs[catalogName].IndexTTL,
		func() (*repo.IndexFile, error) {
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
		},
	)
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

// Helm pull oci://<registry>/<package>:<version>
func (h *HelmPackageRepository) getOCIPackage(
	cfg env.CatalogConfig,
	name string,
) (domain.Package, error) {
	p, err := findOCIPackage(cfg, name)
	if err != nil {
		return domain.Package{}, err
	}

	if len(p.Versions) == 0 {
		return domain.Package{CatalogID: cfg.ID, Name: p.Name, RepoURL: cfg.Location}, nil
	}

	return h.ociPkgs.Get(cfg.ID+"/"+name, ociPackageTTL, func() (domain.Package, error) {
		chartRef := fmt.Sprintf("%s/%s", strings.TrimSuffix(cfg.Location, "/"), name)

		ociCfg := &action.Configuration{RegistryClient: h.registryClient}
		act := action.NewInstall(ociCfg)
		act.Version = p.Versions[0]

		chartPath, err := act.LocateChart(chartRef, h.settings)
		if err != nil {
			return domain.Package{}, fmt.Errorf("locating OCI chart %q: %w", chartRef, err)
		}

		raw, err := loader.Load(chartPath)
		if err != nil {
			return domain.Package{}, fmt.Errorf("loading OCI chart %q: %w", chartRef, err)
		}
		ch, ok := raw.(*chartv2.Chart)
		if !ok {
			return domain.Package{}, fmt.Errorf("unexpected chart type %T", raw)
		}

		return domain.Package{
			CatalogID:   cfg.ID,
			Name:        p.Name,
			Description: ch.Metadata.Description,
			HomeUrl:     tools.MustParseURL(ch.Metadata.Home),
			IconUrl:     tools.MustParseURL(ch.Metadata.Icon),
			RepoURL:     cfg.Location,
		}, nil
	})
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
