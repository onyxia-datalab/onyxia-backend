package helm

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"k8s.io/client-go/rest"
)

type Helm struct {
	cfg      *action.Configuration
	settings *cli.EnvSettings
	global   ports.HelmStartCallbacks
}

var _ ports.HelmReleasesGateway = (*Helm)(nil)

func NewReleaseGtw(
	k8sConfig *rest.Config,
	global ports.HelmStartCallbacks,
) (*Helm, error) {

	settings := cli.New()

	cfg := new(action.Configuration)
	err := cfg.Init(
		&StaticRESTClientGetter{config: k8sConfig},
		settings.Namespace(),
		"secret",
		slog.Debug,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init Helm config: %w", err)
	}

	return &Helm{
		cfg:      cfg,
		settings: settings,
		global:   global,
	}, nil
}

// StartInstall starts a helm install operation in background
func (i *Helm) StartInstall(
	ctx context.Context,
	releaseName string,
	pkg domain.PackageVersion,
	vals map[string]interface{},
	opts ports.HelmStartOptions,
) error {

	if releaseName == "" {
		return fmt.Errorf("releaseName is required")
	}

	chartRef := pkg.ChartRef()

	act := action.NewInstall(i.cfg)
	act.ReleaseName = releaseName
	act.Namespace = i.settings.Namespace()
	act.Version = pkg.Version

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
			slog.String("namespace", i.settings.Namespace()),
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
