package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

// ReleaseState is the minimal release information needed for state derivation.
type ReleaseState struct {
	Exists    bool
	Suspended bool
	// Status is empty when Exists is false.
	Status ReleaseStatus
	// Message is the deployment tool's description of the last operation,
	// e.g. why it failed.
	Message string
}

// ReleaseStatus is the phase of a release, as reported by the deployment
// tool and translated by the adapter.
type ReleaseStatus string

const (
	// ReleaseStatusPending: an install, upgrade or rollback is in progress.
	ReleaseStatusPending ReleaseStatus = "pending"
	// ReleaseStatusDeployed: the manifests are applied; whether the service
	// is up depends on its workloads.
	ReleaseStatusDeployed ReleaseStatus = "deployed"
	// ReleaseStatusFailed: the last operation on the release failed.
	ReleaseStatusFailed ReleaseStatus = "failed"
	// ReleaseStatusUninstalling: the release is being removed.
	ReleaseStatusUninstalling ReleaseStatus = "uninstalling"
	// ReleaseStatusUnknown: the deployment tool doesn't know the state.
	ReleaseStatusUnknown ReleaseStatus = "unknown"
)

type ReleaseGateway interface {
	// StartInstall starts the install in the background and returns as soon
	// as the chart is resolved; the install's outcome is logged and observed
	// through the release state.
	StartInstall(
		ctx context.Context,
		namespace string,
		releaseName string,
		pkg *domain.Package,
		version string,
		vals map[string]interface{},
	) error

	// SuspendRelease scales all Deployments and StatefulSets of a release to 0.
	SuspendRelease(ctx context.Context, namespace, releaseName string) error

	// ResumeRelease restores the replica counts saved during SuspendRelease.
	ResumeRelease(ctx context.Context, namespace, releaseName string) error

	// UninstallRelease removes the Helm release from the namespace.
	UninstallRelease(ctx context.Context, namespace, releaseName string) error

	// GetReleaseState returns the state of one release; Exists is false when
	// there is no such release.
	GetReleaseState(ctx context.Context, namespace, releaseName string) (ReleaseState, error)

	// ListReleaseStates returns the state of every release of the namespace,
	// by release name, in a single call. Absent releases are not in the map.
	ListReleaseStates(ctx context.Context, namespace string) (map[string]ReleaseState, error)
}
