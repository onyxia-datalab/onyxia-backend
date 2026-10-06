package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

// ClusterChangeKind tells what changed in the cluster for a release.
type ClusterChangeKind string

const (
	// ClusterChangeRelease: the release state changed (ReleaseGateway).
	ClusterChangeRelease ClusterChangeKind = "release"
	// ClusterChangePods: a pod of the release was created, updated or
	// deleted (WorkloadStateGateway).
	ClusterChangePods ClusterChangeKind = "pods"
	// ClusterChangeQuota: the namespace's quota usage changed (ProjectQuotaReader).
	ClusterChangeQuota ClusterChangeKind = "quota"
	// ClusterChangeProgress: the cluster reported a step of the release's
	// workloads, carried by Progress.
	ClusterChangeProgress ClusterChangeKind = "progress"
)

// ClusterChange notifies that something changed for a release. Except for
// progress, it carries no state: the receiver reads the new state through
// the other gateways, so it never acts on a partial or reordered view.
type ClusterChange struct {
	Kind     ClusterChangeKind
	Progress *domain.ServiceProgress
}

// ClusterWatcher watches the cluster state of one release. The watch
// is pushed by the cluster (no polling) and scoped to the release and its
// namespace's quota; it lasts only as long as ctx.
type ClusterWatcher interface {
	// Watch starts watching releaseID in namespace and returns once the
	// watch is established, so that no change made after the call is
	// missed. The channel is closed when ctx is done or the watch breaks.
	Watch(ctx context.Context, namespace, releaseID string) (<-chan ClusterChange, error)
}
