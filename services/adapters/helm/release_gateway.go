package helm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart/loader"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/release"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage/driver"
	"k8s.io/client-go/rest"
	sigsyaml "sigs.k8s.io/yaml"
)

type Helm struct {
	settings   *cli.EnvSettings
	global     ports.InstallCallbacks
	restConfig *rest.Config
	helmClient *Client
}

var _ ports.ReleaseGateway = (*Helm)(nil)

func NewReleaseGtw(
	k8sConfig *rest.Config,
	client *Client,
	global ports.InstallCallbacks,
) (*Helm, error) {

	return &Helm{
		settings:   client.Settings,
		global:     global,
		restConfig: k8sConfig,
		helmClient: client,
	}, nil
}

// cfgForNamespace creates a Helm action.Configuration scoped to the given namespace.
func (i *Helm) cfgForNamespace(namespace string) (*action.Configuration, error) {
	cfg := new(action.Configuration)
	if err := cfg.Init(&StaticRESTClientGetter{config: i.restConfig}, namespace, "secret"); err != nil {
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
	opts ports.InstallOptions,
) error {

	if releaseName == "" {
		return fmt.Errorf("releaseName is required")
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
		act.RepoURL = pkg.RepoURL
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

	// Merge values (env/flags + caller vals)
	valMap, err := (&values.Options{}).MergeValues(getter.All(i.settings))
	if err != nil {
		return fmt.Errorf("merging values: %w", err)
	}
	for k, v := range vals {
		valMap[k] = v
	}

	//background operation
	go func() {
		slog.InfoContext(ctx, "helm install started",
			slog.String("release", releaseName),
			slog.String("chart", chartRef),
			slog.String("chartPath", chartPath),
			slog.String("namespace", namespace),
			slog.Bool("disableHooks", act.DisableHooks),
			slog.Duration("timeout", act.Timeout),
		)
		i.global.OnStart(releaseName, chartRef)
		opts.Callbacks.OnStart(releaseName, chartRef)
		_, runErr := act.RunWithContext(ctx, chart, valMap)
		if runErr != nil {
			slog.ErrorContext(ctx, "helm install failed",
				slog.String("release", releaseName),
				slog.String("chart", chartRef),
				slog.Any("error", runErr),
			)
			i.global.OnError(releaseName, chartRef, runErr)
			opts.Callbacks.OnError(releaseName, chartRef, runErr)
			return
		}
		slog.InfoContext(ctx, "helm install completed",
			slog.String("release", releaseName),
			slog.String("chart", chartRef),
		)
		i.global.OnSuccess(releaseName, chartRef)
		opts.Callbacks.OnSuccess(releaseName, chartRef)
	}()

	return nil
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

// UninstallRelease is not yet implemented.
func (i *Helm) UninstallRelease(_ context.Context, _, _ string) error {
	return nil
}

func (i *Helm) toggleSuspend(ctx context.Context, namespace, releaseName string, suspend bool) error {
	cfg, err := i.cfgForNamespace(namespace)
	if err != nil {
		return err
	}

	rel, err := action.NewGet(cfg).Run(releaseName)
	if err != nil {
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
	status := ""
	if r, ok := rel.(*releasev1.Release); ok {
		if global, ok := r.Config["global"].(map[string]interface{}); ok {
			if v, ok := global["suspend"].(bool); ok {
				suspended = v
			}
		}
		if r.Info != nil {
			status = string(r.Info.Status)
		}
	}

	return ports.ReleaseState{Exists: true, Suspended: suspended, Status: status}, nil
}
