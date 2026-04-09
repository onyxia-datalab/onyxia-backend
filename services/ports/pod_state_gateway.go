package ports

import "context"

// PodErrorReason identifies the root cause of a pod failure.
type PodErrorReason string

const (
	PodErrorReasonCrashLoop       PodErrorReason = "crash_loop"
	PodErrorReasonOOMKilled       PodErrorReason = "oom_killed"
	PodErrorReasonImagePull       PodErrorReason = "image_pull"
	PodErrorReasonConfigError     PodErrorReason = "config_error"
	PodErrorReasonUnschedulable   PodErrorReason = "unschedulable"
	PodErrorReasonReadinessFailed PodErrorReason = "readiness_failed"
)

// PodInfo is the minimal pod state needed to derive the service status.
type PodInfo struct {
	Name         string
	Ready        bool
	ErrorReason  PodErrorReason // empty string means no error
	RestartCount int32
	ExitCode     int32
	Image        string
	Message      string
	Limit        string
}

// WorkloadStateGateway provides Kubernetes workload state for a Helm release.
// Resources are selected via the standard Helm label (app.kubernetes.io/instance).
type WorkloadStateGateway interface {
	// GetPodsForRelease returns pod-level detail for error diagnosis (used by GetService).
	GetPodsForRelease(ctx context.Context, namespace, releaseID string) ([]PodInfo, error)

	// GetWorkloadReadiness returns true when all Deployments and StatefulSets for the
	// release have their desired replica count ready (used by ListServices).
	GetWorkloadReadiness(ctx context.Context, namespace, releaseID string) (bool, error)
}
