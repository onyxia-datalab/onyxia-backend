package helm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/loader"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/release"
	"helm.sh/helm/v4/pkg/release/common"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage/driver"
	"k8s.io/client-go/rest"
	sigsyaml "sigs.k8s.io/yaml"
)

type Helm struct {
	settings           *cli.EnvSettings
	catalogs           map[string]env.CatalogConfig
	restGetter         *StaticRESTClientGetter
	helmClient         *Client
	configForNamespace func(string) (*action.Configuration, error)
	// installs tracks the background installs started by StartInstall so a
	// graceful shutdown can wait for them (see WaitForInstalls).
	installs sync.WaitGroup
}

var _ ports.ReleaseGateway = (*Helm)(nil)

func NewReleaseGtw(
	k8sConfig *rest.Config,
	client *Client,
	catalogs []env.CatalogConfig,
) *Helm {
	catalogMap := make(map[string]env.CatalogConfig, len(catalogs))
	for _, c := range catalogs {
		catalogMap[c.ID] = c
	}

	return &Helm{
		settings:   client.Settings,
		catalogs:   catalogMap,
		restGetter: NewStaticRESTClientGetter(k8sConfig),
		helmClient: client,
	}
}

// cfgForNamespace creates a Helm action.Configuration scoped to the given namespace.
func (i *Helm) cfgForNamespace(namespace string) (*action.Configuration, error) {
	if i.configForNamespace != nil {
		return i.configForNamespace(namespace)
	}

	cfg := new(action.Configuration)
	if err := cfg.Init(i.restGetter, namespace, "secret"); err != nil {
		return nil, fmt.Errorf("init helm config for namespace %q: %w", namespace, err)
	}
	cfg.RegistryClient = i.helmClient.RegistryClient
	return cfg, nil
}

// StartInstall starts a helm install operation in background
func (i *Helm) StartInstall(
	ctx context.Context,
	namespace string,
	releaseName string,
	pkg *domain.Package,
	version string,
	vals map[string]interface{},
) error {

	if releaseName == "" {
		return fmt.Errorf("releaseName is required")
	}
	if pkg == nil {
		return fmt.Errorf("package is required")
	}

	cfg, err := i.cfgForNamespace(namespace)
	if err != nil {
		return err
	}

	act := action.NewInstall(cfg)
	act.ReleaseName = releaseName
	act.Namespace = namespace
	act.Version = version

	var chartRef string
	if pkg.ChartRef != "" {
		chartRef = pkg.ChartRef
	} else {
		if cfg, ok := i.catalogs[pkg.CatalogID]; ok {
			applyRepoAccess(&act.ChartPathOptions, cfg)
		} else {
			act.RepoURL = pkg.RepoURL
		}
		chartRef = pkg.Name
	}

	chartPath, err := act.LocateChart(chartRef, i.settings)
	if err != nil {
		return fmt.Errorf("locating chart %q: %w", chartRef, err)
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("loading chart: %w", err)
	}

	// The HTTP request ends as soon as this method returns. Preserve its values
	// without letting request cancellation abort the asynchronous Helm action.
	installCtx := context.WithoutCancel(ctx)

	i.installs.Go(func() {
		slog.InfoContext(installCtx, "helm install started",
			slog.String("release", releaseName),
			slog.String("chart", chartRef),
			slog.String("chartPath", chartPath),
			slog.String("namespace", namespace),
			slog.Bool("disableHooks", act.DisableHooks),
			slog.Duration("timeout", act.Timeout),
		)
		_, runErr := act.RunWithContext(installCtx, chart, vals)
		if runErr != nil {
			slog.ErrorContext(installCtx, "helm install failed",
				slog.String("release", releaseName),
				slog.String("chart", chartRef),
				slog.Any("error", runErr),
			)
			return
		}
		slog.InfoContext(installCtx, "helm install completed",
			slog.String("release", releaseName),
			slog.String("chart", chartRef),
		)
	})

	return nil
}

// WaitForInstalls blocks until every background install started by
// StartInstall has finished, or ctx is done. An install interrupted by the
// process exit leaves its release in pending-install.
func (i *Helm) WaitForInstalls(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		i.installs.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("background helm installs still running: %w", ctx.Err())
	}
}

// SuspendRelease runs helm upgrade --reuse-values with global.suspend=true.
// Returns domain.ErrNotSupported if the chart does not expose global.suspend.
func (i *Helm) SuspendRelease(ctx context.Context, namespace, releaseName string) error {
	return i.toggleSuspend(ctx, namespace, releaseName, true)
}

// ResumeRelease runs helm upgrade --reuse-values with global.suspend=false.
// Returns domain.ErrNotSupported if the chart does not expose global.suspend.
func (i *Helm) ResumeRelease(ctx context.Context, namespace, releaseName string) error {
	return i.toggleSuspend(ctx, namespace, releaseName, false)
}

// UninstallRelease removes a Helm release. Missing releases are ignored so a
// stale Onyxia secret (a Ghost service) can still be cleaned up by the use case.
func (i *Helm) UninstallRelease(ctx context.Context, namespace, releaseName string) error {
	if releaseName == "" {
		return fmt.Errorf("releaseName is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	cfg, err := i.cfgForNamespace(namespace)
	if err != nil {
		return err
	}

	act := action.NewUninstall(cfg)
	if _, err := act.Run(releaseName); errors.Is(err, driver.ErrReleaseNotFound) {
		return nil
	} else if err != nil {
		return fmt.Errorf("uninstall release %q: %w", releaseName, err)
	}

	slog.InfoContext(ctx, "helm release uninstalled",
		slog.String("release", releaseName),
		slog.String("namespace", namespace),
	)
	return nil
}

func (i *Helm) toggleSuspend(ctx context.Context, namespace, releaseName string, suspend bool) error {
	cfg, err := i.cfgForNamespace(namespace)
	if err != nil {
		return err
	}

	rel, err := action.NewGet(cfg).Run(releaseName)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return fmt.Errorf("%w: release %q", domain.ErrNotFound, releaseName)
		}
		return fmt.Errorf("get release %q: %w", releaseName, err)
	}

	accessor, ok := rel.(release.Accessor)
	if !ok {
		return fmt.Errorf("unexpected release type for %q", releaseName)
	}

	ch := accessor.Chart()
	chartObj, ok := ch.(*chartv2.Chart)
	if !ok {
		return fmt.Errorf("unexpected chart type for release %q", releaseName)
	}

	if !globalSuspendSupported(chartObj.Values) {
		return fmt.Errorf("%w: chart %q does not expose global.suspend", domain.ErrNotSupported, chartObj.Name())
	}

	act := action.NewUpgrade(cfg)
	act.ReuseValues = true
	act.Namespace = namespace

	newVals := map[string]interface{}{
		"global": map[string]interface{}{"suspend": suspend},
	}

	if _, err := act.RunWithContext(ctx, releaseName, ch, newVals); err != nil {
		return fmt.Errorf("helm upgrade (suspend=%v) on %q: %w", suspend, releaseName, err)
	}

	slog.InfoContext(ctx, "service suspend toggled",
		slog.String("release", releaseName),
		slog.String("namespace", namespace),
		slog.Bool("suspend", suspend),
	)
	return nil
}

// globalSuspendSupported returns true if the chart's default values contain a
// global.suspend key, indicating the chart handles suspension natively.
func globalSuspendSupported(chartValues map[string]interface{}) bool {
	global, ok := chartValues["global"].(map[string]interface{})
	if !ok {
		return false
	}
	_, ok = global["suspend"]
	return ok
}

// GetReleaseResources parses the release manifest and returns all declared resources.
func (h *Helm) GetReleaseResources(
	ctx context.Context,
	namespace, releaseName string,
) ([]ports.ManifestResource, error) {
	cfg, err := h.cfgForNamespace(namespace)
	if err != nil {
		return nil, err
	}

	rel, err := action.NewGet(cfg).Run(releaseName)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get release %q: %w", releaseName, err)
	}

	r, ok := rel.(*releasev1.Release)
	if !ok {
		return nil, fmt.Errorf("unexpected release type for %q", releaseName)
	}

	return parseManifestResources(r.Manifest), nil
}

// parseManifestResources splits a multi-document YAML manifest and extracts
// the Kind and metadata.name of each resource.
func parseManifestResources(manifest string) []ports.ManifestResource {
	var resources []ports.ManifestResource

	type meta struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}

	for _, doc := range strings.Split(manifest, "\n---") {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}
		var m meta
		if err := sigsyaml.Unmarshal([]byte(doc), &m); err != nil || m.Kind == "" || m.Metadata.Name == "" {
			continue
		}
		resources = append(resources, ports.ManifestResource{Kind: m.Kind, Name: m.Metadata.Name})
	}

	return resources
}

// GetReleaseState returns whether the release exists and whether global.suspend is true.
// Returns ReleaseState{Exists: false} (no error) when the release is not found.
func (h *Helm) GetReleaseState(
	ctx context.Context,
	namespace, releaseName string,
) (ports.ReleaseState, error) {
	cfg, err := h.cfgForNamespace(namespace)
	if err != nil {
		return ports.ReleaseState{}, err
	}

	rel, err := action.NewGet(cfg).Run(releaseName)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return ports.ReleaseState{Exists: false}, nil
		}
		return ports.ReleaseState{}, err
	}

	suspended := false
	status := ports.ReleaseStatusUnknown
	if r, ok := rel.(*releasev1.Release); ok {
		if global, ok := r.Config["global"].(map[string]interface{}); ok {
			if v, ok := global["suspend"].(bool); ok {
				suspended = v
			}
		}
		if r.Info != nil {
			if r.Info.Status == common.StatusUninstalled {
				// Uninstalled with --keep-history: only the history remains.
				return ports.ReleaseState{Exists: false}, nil
			}
			status = releaseStatus(r.Info.Status)
		}
	}

	return ports.ReleaseState{Exists: true, Suspended: suspended, Status: status}, nil
}

// releaseStatus translates a Helm release status into the port's vocabulary.
func releaseStatus(s common.Status) ports.ReleaseStatus {
	switch s {
	case common.StatusPendingInstall, common.StatusPendingUpgrade, common.StatusPendingRollback:
		return ports.ReleaseStatusPending
	case common.StatusDeployed, common.StatusSuperseded:
		return ports.ReleaseStatusDeployed
	case common.StatusFailed:
		return ports.ReleaseStatusFailed
	case common.StatusUninstalling:
		return ports.ReleaseStatusUninstalling
	default:
		return ports.ReleaseStatusUnknown
	}
}
