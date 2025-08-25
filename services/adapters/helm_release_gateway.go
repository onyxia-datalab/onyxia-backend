package adapters

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
)

type Helm struct {
	cfg      *action.Configuration
	settings *cli.EnvSettings
	global   ports.HelmStartCallbacks
}

var _ ports.HelmReleasesGateway = (*Helm)(nil)

func New(
	cfg *action.Configuration,
	settings *cli.EnvSettings,
	global ports.HelmStartCallbacks,
) *Helm {
	return &Helm{cfg: cfg, settings: settings, global: global}
}

// StartInstall starts a helm install operation in background
func (i *Helm) StartInstall(
	ctx context.Context,
	releaseName string,
	chartRef string,
	vals map[string]interface{},
	opts ports.HelmStartOptions,
) error {
	if releaseName == "" || chartRef == "" {
		return fmt.Errorf("releaseName and chartRef are required")
	}

	act := action.NewInstall(i.cfg)
	act.ReleaseName = releaseName
	act.Namespace = i.settings.Namespace()

	cp, err := act.LocateChart(chartRef, i.settings)
	if err != nil {
		return fmt.Errorf("locating chart %q: %w", chartRef, err)
	}
	ch, err := loader.Load(cp)
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

		i.global.OnStart(releaseName, chartRef)
		opts.Callbacks.OnStart(releaseName, chartRef)
		slog.InfoContext(ctx, "helm install started",
			"release", releaseName,
			"chart", chartRef,
			"chartPath", cp,
			"namespace", i.settings.Namespace(),
			"disableHooks", act.DisableHooks,
			"timeout", act.Timeout,
		)
		_, runErr := act.RunWithContext(ctx, ch, valMap)
		if runErr != nil {
			i.global.OnError(releaseName, chartRef, runErr)
			opts.Callbacks.OnError(releaseName, chartRef, runErr)
			slog.ErrorContext(ctx, "helm install failed",
				"release", releaseName,
				"chart", chartRef,
				"err", runErr,
			)
			return
		}
		i.global.OnSuccess(releaseName, chartRef)
		opts.Callbacks.OnSuccess(releaseName, chartRef)
		slog.InfoContext(ctx, "helm install completed",
			"release", releaseName,
			"chart", chartRef,
		)
	}()

	return nil
}
