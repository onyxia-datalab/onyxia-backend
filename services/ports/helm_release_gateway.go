package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

type HelmStartCallbacks struct {
	OnStart   func(release, chart string)
	OnSuccess func(release, chart string)
	OnError   func(release, chart string, err error)
}

type HelmStartOptions struct {
	Callbacks HelmStartCallbacks // per-call callbacks (optional)
}

type HelmReleasesGateway interface {
	// Start a Helm install in the background and returns immediately.
	StartInstall(
		ctx context.Context,
		releaseName string,
		pkg domain.PackageVersion,
		vals map[string]interface{},
		opts HelmStartOptions,
	) error
}
