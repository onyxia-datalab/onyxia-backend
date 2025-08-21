package ports

import (
	"context"
	"time"
)

type HelmInstallCallbacks struct {
	OnStart   func(release, chart string)
	OnSuccess func(release, chart string)
	OnError   func(release, chart string, err error)
}

type HelmInstallOptions struct {
	Version   string               // chart version if installing from a repo
	Timeout   time.Duration        // execution timeout (does not imply readiness wait)
	Callbacks HelmInstallCallbacks // per-call callbacks (optional)
}

type Helm interface {
	// Start launches a Helm install in the background and returns immediately.
	// Implementations should fail fast on invalid chart/values before spawning work.
	StartInstall(
		ctx context.Context,
		releaseName string,
		chartRef string,
		vals map[string]interface{},
		opts HelmInstallOptions,
	) error
}
