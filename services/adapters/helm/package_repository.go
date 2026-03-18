package helm

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/onyxia-datalab/onyxia-backend/internal/tools"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/repo"
)

var _ ports.PackageRepository = (*HelmPackageRepository)(nil)

type HelmPackageRepository struct {
	repos          map[string]*repo.ChartRepository
	catalogs       map[string]env.CatalogConfig
	getters        getter.Providers
	versionFilters map[string]versionFilter
}

type Option func(*cli.EnvSettings)

// WithHelmSettings allows tests or external callers to override Helm CLI settings.
func WithHelmSettings(custom *cli.EnvSettings) Option {
	return func(s *cli.EnvSettings) {
		if custom != nil {
			*s = *custom // copy values to avoid pointer aliasing
		}
	}
}

func NewPackageRepository(
	cfgs []env.CatalogConfig, opts ...Option,
) (*HelmPackageRepository, error) {
	settings := cli.New()

	for _, opt := range opts {
		opt(settings)
	}

	repos := make(map[string]*repo.ChartRepository)
	catalogs := make(map[string]env.CatalogConfig)
	filters := make(map[string]versionFilter)
	getters := getter.All(settings)

	for _, cfg := range cfgs {
		catalogs[cfg.ID] = cfg

		if cfg.Type != env.CatalogTypeHelmRepo {
			continue
		}

		f, err := versionFilterFrom(cfg)
		if err != nil {
			return nil, err
		}
		filters[cfg.ID] = f

		entry := &repo.Entry{
			Name:                  cfg.ID,
			URL:                   cfg.Location,
			Username:              tools.Deref(cfg.Username),
			Password:              tools.Deref(cfg.Password),
			InsecureSkipTLSverify: cfg.SkipTLSVerify,
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
		repos:          repos,
		catalogs:       catalogs,
		getters:        getters,
		versionFilters: filters,
	}, nil
}

func (h *HelmPackageRepository) ListPackages(
	ctx context.Context,
	cfg env.CatalogConfig,
) ([]domain.Package, error) {
	slog.InfoContext(ctx, "")
	switch cfg.Type {
	case env.CatalogTypeHelmRepo:
		return h.listHelmPackages(ctx, cfg)
	case env.CatalogTypeOCI:
		return h.listOCIPackages(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported catalog type: %v", cfg.Type)
	}
}

func (h *HelmPackageRepository) GetPackage(
	ctx context.Context,
	cfg env.CatalogConfig,
	name string,
) (*domain.PackageRef, error) {
	switch cfg.Type {
	case env.CatalogTypeHelmRepo:
		return h.getHelmPackage(ctx, cfg, name)
	case env.CatalogTypeOCI:
		return h.getOCIPackage(ctx, cfg, name)
	default:
		return nil, fmt.Errorf("unsupported catalog type: %v", cfg.Type)
	}
}

func (h *HelmPackageRepository) ResolvePackage(
	ctx context.Context,
	catalogID, pkgName, version string,
) (domain.PackageVersion, error) {
	cfg, ok := h.catalogs[catalogID]
	if !ok {
		return domain.PackageVersion{}, fmt.Errorf("catalog %q not found", catalogID)
	}
	if cfg.Type == env.CatalogTypeOCI {
		return resolveOCIPackage(cfg, pkgName, version)
	}

	slog.InfoContext(ctx, "Resolving Helm package version",
		slog.String("catalog", catalogID),
		slog.String("package", pkgName),
		slog.String("version", version),
	)

	cr, idx, err := h.loadHelmIndex(catalogID)
	if err != nil {
		return domain.PackageVersion{}, err
	}

	versions, ok := idx.Entries[pkgName]
	if !ok {
		return domain.PackageVersion{}, fmt.Errorf(
			"chart %q not found in catalog %q",
			pkgName,
			catalogID,
		)
	}

	for _, v := range versions {
		if v.Version == version {
			return domain.PackageVersion{
				Package: domain.Package{
					Name:      pkgName,
					CatalogID: catalogID,
				},
				Version: version,
				RepoURL: cr.Config.URL,
			}, nil
		}
	}

	return domain.PackageVersion{}, fmt.Errorf(
		"version %q not found for chart %q in catalog %q",
		version, pkgName, catalogID,
	)
}

func (h *HelmPackageRepository) loadHelmIndex(catalogID string) (*repo.ChartRepository, *repo.IndexFile, error) {
	cr, ok := h.repos[catalogID]
	if !ok {
		return nil, nil, fmt.Errorf("unknown Helm catalog: %s", catalogID)
	}
	if _, err := cr.DownloadIndexFile(); err != nil {
		return nil, nil, fmt.Errorf("fetching Helm index: %w", err)
	}
	indexPath := filepath.Join(cr.CachePath, helmpath.CacheIndexFile(catalogID))
	idx, err := repo.LoadIndexFile(indexPath)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing Helm index: %w", err)
	}
	return cr, idx, nil
}

func (h *HelmPackageRepository) listHelmPackages(
	ctx context.Context,
	cfg env.CatalogConfig,
) ([]domain.Package, error) {

	_, idx, err := h.loadHelmIndex(cfg.ID)
	if err != nil {
		return nil, err
	}

	pkgs := make([]domain.Package, 0, len(idx.Entries))
	for name, versions := range idx.Entries {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if len(versions) == 0 || isExcluded(cfg.Excluded, name) {
			continue
		}

		latest := versions[0]
		pkgs = append(pkgs, domain.Package{
			CatalogID:   cfg.ID,
			Name:        name,
			Description: latest.Description,
			HomeUrl:     tools.MustParseURL(latest.Home),
			IconUrl:     tools.MustParseURL(latest.Icon),
		})
	}

	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

func (h *HelmPackageRepository) getHelmPackage(
	ctx context.Context,
	cfg env.CatalogConfig,
	name string,
) (*domain.PackageRef, error) {

	_, idx, err := h.loadHelmIndex(cfg.ID)
	if err != nil {
		return nil, err
	}

	versions, ok := idx.Entries[name]
	if !ok || len(versions) == 0 {
		return nil, fmt.Errorf("%w: chart %q not found in catalog %q", domain.ErrNotFound, name, cfg.ID)
	}

	latest := versions[0]
	return &domain.PackageRef{
		Package: domain.Package{
			CatalogID:   cfg.ID,
			Name:        name,
			Description: latest.Description,
			HomeUrl:     tools.MustParseURL(latest.Home),
			IconUrl:     tools.MustParseURL(latest.Icon),
		},
		Versions: h.versionFilters[cfg.ID].apply(extractVersions(versions)),
	}, nil
}

func (h *HelmPackageRepository) GetPackageSchema(
	ctx context.Context,
	cfg env.CatalogConfig,
	packageName string,
	version string,
) ([]byte, error) {
	switch cfg.Type {
	case env.CatalogTypeHelmRepo:
		return h.getHelmPackageSchema(ctx, cfg, packageName, version)
	case env.CatalogTypeOCI:
		return h.getOCIPackageSchema(ctx, cfg, packageName, version)
	default:
		return nil, fmt.Errorf("unsupported catalog type: %v", cfg.Type)
	}
}

func (h *HelmPackageRepository) getHelmPackageSchema(
	ctx context.Context,
	cfg env.CatalogConfig,
	packageName string,
	version string,
) ([]byte, error) {
	cr, idx, err := h.loadHelmIndex(cfg.ID)
	if err != nil {
		return nil, err
	}

	entries, ok := idx.Entries[packageName]
	if !ok || len(entries) == 0 {
		return nil, fmt.Errorf("%w: chart %q not found in catalog %q", domain.ErrNotFound, packageName, cfg.ID)
	}

	var target *repo.ChartVersion
	for _, v := range entries {
		if v.Version == version {
			target = v
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("%w: version %q not found for chart %q", domain.ErrNotFound, version, packageName)
	}

	chartURL, err := repo.ResolveReferenceURL(cr.Config.URL, target.URLs[0])
	if err != nil {
		return nil, fmt.Errorf("resolving chart URL: %w", err)
	}
	ch, err := h.pullChart(chartURL,
		getter.WithURL(cr.Config.URL),
		getter.WithInsecureSkipVerifyTLS(cfg.SkipTLSVerify),
		getter.WithTLSClientConfig(cr.Config.CertFile, cr.Config.KeyFile, cr.Config.CAFile),
		getter.WithBasicAuth(cr.Config.Username, cr.Config.Password),
		getter.WithPassCredentialsAll(cr.Config.PassCredentialsAll),
	)
	if err != nil {
		return nil, fmt.Errorf("pulling chart: %w", err)
	}

	return extractSchema(ch), nil
}

func (h *HelmPackageRepository) getOCIPackageSchema(
	ctx context.Context,
	cfg env.CatalogConfig,
	packageName string,
	version string,
) ([]byte, error) {
	var pkg *env.OCIPackage
	for _, p := range cfg.Packages {
		if p.Name == packageName {
			pkg = &p
			break
		}
	}
	if pkg == nil {
		return nil, fmt.Errorf("%w: package %q not found in OCI catalog %q", domain.ErrNotFound, packageName, cfg.ID)
	}

	base := strings.TrimSuffix(cfg.Location, "/")
	ref := fmt.Sprintf("%s/%s", base, strings.TrimPrefix(packageName, "/"))
	ch, err := h.pullChart(ref,
		getter.WithURL(ref),
		getter.WithTagName(version),
		getter.WithInsecureSkipVerifyTLS(cfg.SkipTLSVerify),
		getter.WithTLSClientConfig("", "", tools.Deref(cfg.CAFile)),
		getter.WithBasicAuth(tools.Deref(cfg.Username), tools.Deref(cfg.Password)),
	)
	if err != nil {
		return nil, fmt.Errorf("pulling OCI chart: %w", err)
	}

	return extractSchema(ch), nil
}

func extractSchema(ch *chart.Chart) []byte {
	if len(ch.Schema) == 0 {
		return []byte("{}")
	}
	return ch.Schema
}

func (h *HelmPackageRepository) pullChart(chartURL string, opts ...getter.Option) (*chart.Chart, error) {
	u, err := url.Parse(chartURL)
	if err != nil {
		return nil, fmt.Errorf("invalid chart URL: %w", err)
	}
	g, err := h.getters.ByScheme(u.Scheme)
	if err != nil {
		return nil, fmt.Errorf("no getter for scheme %q: %w", u.Scheme, err)
	}
	buf, err := g.Get(chartURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("downloading chart: %w", err)
	}
	return loader.LoadArchive(bytes.NewReader(buf.Bytes()))
}

func (h *HelmPackageRepository) listOCIPackages(
	ctx context.Context,
	cfg env.CatalogConfig,
) ([]domain.Package, error) {
	pkgs := make([]domain.Package, 0, len(cfg.Packages))
	for _, p := range cfg.Packages {
		if isExcluded(cfg.Excluded, p.Name) {
			continue
		}
		pkgs = append(pkgs, domain.Package{
			CatalogID: cfg.ID,
			Name:      p.Name,
		})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

func (h *HelmPackageRepository) getOCIPackage(
	ctx context.Context,
	cfg env.CatalogConfig,
	name string,
) (*domain.PackageRef, error) {

	var pkg *env.OCIPackage
	for _, p := range cfg.Packages {
		if p.Name == name {
			pkg = &p
			break
		}
	}
	if pkg == nil {
		return nil, fmt.Errorf("package %q not found in OCI catalog %q", name, cfg.ID)
	}

	base := strings.TrimSuffix(cfg.Location, "/")
	result := domain.PackageRef{
		Package: domain.Package{
			CatalogID: cfg.ID,
			Name:      name,
		},
		Versions: pkg.Versions,
	}

	for _, version := range pkg.Versions {
		ref := fmt.Sprintf("%s/%s", base, strings.TrimPrefix(name, "/"))
		ch, err := h.pullChart(ref,
			getter.WithURL(ref),
			getter.WithTagName(version),
			getter.WithInsecureSkipVerifyTLS(cfg.SkipTLSVerify),
			getter.WithTLSClientConfig("", "", tools.Deref(cfg.CAFile)),
			getter.WithBasicAuth(tools.Deref(cfg.Username), tools.Deref(cfg.Password)),
		)
		if err != nil {
			continue
		}
		if ch.Metadata != nil && ch.Metadata.Description != "" {
			result.Description = ch.Metadata.Description
			result.HomeUrl = tools.MustParseURL(ch.Metadata.Home)
			result.IconUrl = tools.MustParseURL(ch.Metadata.Icon)
		}
	}

	return &result, nil
}

func resolveOCIPackage(cfg env.CatalogConfig, pkgName, version string) (domain.PackageVersion, error) {
	for _, p := range cfg.Packages {
		if p.Name != pkgName {
			continue
		}
		for _, v := range p.Versions {
			if v == version {
				return domain.PackageVersion{
					Package: domain.Package{
						Name:      pkgName,
						CatalogID: cfg.ID,
					},
					Version: version,
					RepoURL: cfg.Location,
				}, nil
			}
		}
		return domain.PackageVersion{}, fmt.Errorf(
			"%w: version %q not found for package %q in OCI catalog %q",
			domain.ErrNotFound, version, pkgName, cfg.ID,
		)
	}
	return domain.PackageVersion{}, fmt.Errorf(
		"%w: package %q not found in OCI catalog %q",
		domain.ErrNotFound, pkgName, cfg.ID,
	)
}

func extractVersions(list []*repo.ChartVersion) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if v != nil && v.Version != "" {
			out = append(out, v.Version)
		}
	}
	return out
}

func isExcluded(list []string, name string) bool {
	for _, x := range list {
		if x == name {
			return true
		}
	}
	return false
}
