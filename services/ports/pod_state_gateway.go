package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

// PodInfo is the minimal pod state needed to derive the service status.
type PodInfo struct {
	Name  string
	Ready bool
	// Terminating is true once the pod is being deleted: it still runs but
	// no longer counts towards the service's readiness.
	Terminating  bool
	ErrorReason  domain.ServiceErrorReason // empty string means no error
	RestartCount int32
	ExitCode     int32
	Image        string
	Message      string
	Limit        string
}

// WorkloadStateGateway provides the state of the pods of releases. Only live
// pods are returned: pods that ran to completion (succeeded or failed) are
// left out.
type WorkloadStateGateway interface {
	// GetPodsForRelease returns the live pods of one release.
	GetPodsForRelease(ctx context.Context, namespace, releaseID string) ([]PodInfo, error)

	// ListPodsByRelease returns the live pods of the namespace grouped by
	// release, in a constant number of calls whatever the number of releases.
	ListPodsByRelease(ctx context.Context, namespace string) (map[string][]PodInfo, error)
}
