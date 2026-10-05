package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

// PodInfo is the minimal pod state needed to derive the service status.
type PodInfo struct {
	Name         string
	Ready        bool
	ErrorReason  domain.ServiceErrorReason // empty string means no error
	RestartCount int32
	ExitCode     int32
	Image        string
	Message      string
	Limit        string
}

// WorkloadStateGateway provides Kubernetes workload state.
type WorkloadStateGateway interface {
	// GetPodsForRelease returns pod-level detail for error diagnosis (used by GetService).
	GetPodsForRelease(ctx context.Context, namespace, releaseID string) ([]PodInfo, error)

	// GetWorkloadReadiness snapshots the readiness of the namespace's
	// workloads in a constant number of calls, so that the readiness of many
	// releases can be checked without a request per release.
	GetWorkloadReadiness(ctx context.Context, namespace string) (WorkloadReadiness, error)
}

// WorkloadReadiness is a snapshot of a namespace's workload readiness.
type WorkloadReadiness interface {
	// AllReady reports whether every workload controller among resources is
	// ready. The implementation decides which kinds are controllers; other
	// kinds are ignored. A declared controller that doesn't exist is not ready.
	AllReady(resources []ManifestResource) bool
}

// WorkloadReadinessFunc adapts a function to WorkloadReadiness.
type WorkloadReadinessFunc func(resources []ManifestResource) bool

func (f WorkloadReadinessFunc) AllReady(resources []ManifestResource) bool { return f(resources) }
