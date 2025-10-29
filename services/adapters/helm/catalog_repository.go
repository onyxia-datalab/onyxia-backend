package helm

import (
	"bytes"
	"context"
	"fmt"
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

var _ ports.CatalogRepository = (*HelmCatalogRepository)(nil)

type HelmCatalogRepository struct {
	repos   map[string]*repo.ChartRepository
	getters getter.Providers
}

func NewCatalogRepo(
	settings *cli.EnvSettings,
	cfgs []env.CatalogConfig,
) (*HelmCatalogRepository, error) {

	repos := make(map[string]*repo.ChartRepository)
	getters := getter.All(settings)

	for _, cfg := range cfgs {
		if cfg.Type != env.CatalogTypeHelm {
			continue
		}
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
	}

	return &HelmCatalogRepository{
		repos:   repos,
		getters: getters,
	}, nil
}

func (h *HelmCatalogRepository) ListPackages(
	ctx context.Context,
	cfg env.CatalogConfig,
) ([]domain.Package, error) {
	switch cfg.Type {
	case env.CatalogTypeHelm:
		return h.listHelmPackages(ctx, cfg)
	case env.CatalogTypeOCI:
		return h.listOCIPackages(ctx, cfg)
	default:
		return nil, fmt.Errorf("unsupported catalog type: %v", cfg.Type)
	}
}

func (h *HelmCatalogRepository) GetPackage(
	ctx context.Context,
	cfg env.CatalogConfig,
	name string,
) (*domain.PackageRef, error) {
	switch cfg.Type {
	case env.CatalogTypeHelm:
		return h.getHelmPackage(ctx, cfg, name)
	case env.CatalogTypeOCI:
		return h.getOCIPackage(ctx, cfg, name)
	default:
		return nil, fmt.Errorf("unsupported catalog type: %v", cfg.Type)
	}
}

func (h *HelmCatalogRepository) listHelmPackages(
	ctx context.Context,
	cfg env.CatalogConfig,
) ([]domain.Package, error) {

	cr, ok := h.repos[cfg.ID]
	if !ok {
		return nil, fmt.Errorf("unknown Helm catalog: %s", cfg.ID)
	}

	if _, err := cr.DownloadIndexFile(); err != nil {
		return nil, fmt.Errorf("fetching Helm index: %w", err)
	}

	indexPath := filepath.Join(cr.CachePath, helmpath.CacheIndexFile(cfg.ID))
	idx, err := repo.LoadIndexFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("parsing Helm index: %w", err)
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
			HomeUrl:     mustParseURL(latest.Home),
			IconUrl:     mustParseURL(latest.Icon),
		})
	}

	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

func (h *HelmCatalogRepository) getHelmPackage(
	ctx context.Context,
	cfg env.CatalogConfig,
	name string,
) (*domain.PackageRef, error) {

	cr, ok := h.repos[cfg.ID]
	if !ok {
		return nil, fmt.Errorf("unknown Helm catalog: %s", cfg.ID)
	}

	if _, err := cr.DownloadIndexFile(); err != nil {
		return nil, fmt.Errorf("fetching Helm index: %w", err)
	}

	indexPath := filepath.Join(cr.CachePath, helmpath.CacheIndexFile(cfg.ID))
	idx, err := repo.LoadIndexFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("parsing Helm index: %w", err)
	}

	versions, ok := idx.Entries[name]
	if !ok || len(versions) == 0 {
		return nil, fmt.Errorf("chart %q not found in catalog %q", name, cfg.ID)
	}

	pkg := domain.PackageRef{
		Package: domain.Package{
			CatalogID: cfg.ID,
			Name:      name,
		},
		Versions: extractVersions(versions),
	}

	// Pull each version for full metadata (values.yaml, schema, etc.)
	for _, v := range versions {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		ch, err := h.pullHelmChart(cr, v, cfg)
		if err != nil {
			continue // skip missing version, don't fail the whole call
		}

		// You could extract schema here if needed:
		// schema := findChartFile(ch, "values.schema.json")
		_ = ch
		if ch.Metadata != nil && ch.Metadata.Description != "" {
			pkg.Description = ch.Metadata.Description
			pkg.HomeUrl = mustParseURL(ch.Metadata.Home)
			pkg.IconUrl = mustParseURL(ch.Metadata.Icon)
		}
	}

	return &pkg, nil
}

func (h *HelmCatalogRepository) pullHelmChart(
	chartRepo *repo.ChartRepository,
	version *repo.ChartVersion,
	cfg env.CatalogConfig,
) (*chart.Chart, error) {
	if version == nil || len(version.URLs) == 0 {
		return nil, fmt.Errorf("invalid chart version metadata")
	}

	chartURL, err := repo.ResolveReferenceURL(chartRepo.Config.URL, version.URLs[0])
	if err != nil {
		return nil, fmt.Errorf("resolving chart URL: %w", err)
	}

	buf, err := chartRepo.Client.Get(
		chartURL,
		getter.WithURL(chartRepo.Config.URL),
		getter.WithInsecureSkipVerifyTLS(cfg.SkipTLSVerify),
		getter.WithTLSClientConfig(
			chartRepo.Config.CertFile,
			chartRepo.Config.KeyFile,
			chartRepo.Config.CAFile,
		),
		getter.WithBasicAuth(chartRepo.Config.Username, chartRepo.Config.Password),
		getter.WithPassCredentialsAll(chartRepo.Config.PassCredentialsAll),
	)
	if err != nil {
		return nil, fmt.Errorf("downloading chart: %w", err)
	}

	return loader.LoadArchive(bytes.NewReader(buf.Bytes()))
}

func (h *HelmCatalogRepository) listOCIPackages(
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

func (h *HelmCatalogRepository) getOCIPackage(
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

	oGetter, err := h.getters.ByScheme("oci")
	if err != nil {
		return nil, fmt.Errorf("oci getter unavailable: %w", err)
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
		buf, err := oGetter.Get(
			ref,
			getter.WithURL(ref),
			getter.WithTagName(version),
			getter.WithInsecureSkipVerifyTLS(cfg.SkipTLSVerify),
			getter.WithTLSClientConfig("", "", tools.Deref(cfg.CAFile)),
			getter.WithBasicAuth(tools.Deref(cfg.Username), tools.Deref(cfg.Password)),
		)
		if err != nil {
			continue
		}

		ch, err := loader.LoadArchive(bytes.NewReader(buf.Bytes()))
		if err != nil {
			continue
		}
		if ch.Metadata != nil && ch.Metadata.Description != "" {
			result.Description = ch.Metadata.Description
			result.HomeUrl = mustParseURL(ch.Metadata.Home)
			result.IconUrl = mustParseURL(ch.Metadata.Icon)
		}
	}

	return &result, nil
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

func findChartFile(ch *chart.Chart, name string) []byte {
	for _, f := range ch.Files {
		if f.Name == name {
			return f.Data
		}
	}
	return nil
}

func isExcluded(list []string, name string) bool {
	for _, x := range list {
		if x == name {
			return true
		}
	}
	return false
}

func mustParseURL(s string) (u url.URL) {
	p, _ := url.Parse(strings.TrimSpace(s))
	if p != nil {
		return *p
	}
	return url.URL{}
}
