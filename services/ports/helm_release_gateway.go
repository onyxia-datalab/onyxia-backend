package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

// ManifestResource is a neutral reference to a resource declared in a Helm release manifest.
// Kind and Name are raw values from the manifest — no Kubernetes-specific semantics.
type ManifestResource struct {
	Kind string
	Name string
}

// ReleaseState is the minimal release information needed for state derivation.
type ReleaseState struct {
	Exists    bool
	Suspended bool
	// Status is the Helm release status (e.g. "deployed", "failed", "pending-install").
	// Empty when Exists is false.
	Status string
}

type InstallCallbacks struct {
	OnStart   func(release, chart string)
	OnSuccess func(release, chart string)
	OnError   func(release, chart string, err error)
}

type InstallOptions struct {
	Callbacks InstallCallbacks // per-call callbacks (optional)
}

type ReleaseGateway interface {
	// Start a Helm install in the background and returns immediately.
	StartInstall(
		ctx context.Context,
		namespace string,
		releaseName string,
		pkg *domain.Package,
		version string,
		vals map[string]interface{},
		opts InstallOptions,
	) error

	// SuspendRelease scales all Deployments and StatefulSets of a release to 0.
	SuspendRelease(ctx context.Context, namespace, releaseName string) error

	// ResumeRelease restores the replica counts saved during SuspendRelease.
	ResumeRelease(ctx context.Context, namespace, releaseName string) error

	// UninstallRelease removes the Helm release from the namespace.
	UninstallRelease(ctx context.Context, namespace, releaseName string) error

	// GetReleaseState returns whether the release exists and whether global.suspend is true.
	GetReleaseState(ctx context.Context, namespace, releaseName string) (ReleaseState, error)

	// GetReleaseResources returns all resources declared in the release manifest.
	GetReleaseResources(ctx context.Context, namespace, releaseName string) ([]ManifestResource, error)
}
