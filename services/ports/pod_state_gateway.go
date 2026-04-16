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

// WorkloadStateGateway provides Kubernetes workload state for a set of manifest resources.
type WorkloadStateGateway interface {
	// GetPodsForRelease returns pod-level detail for error diagnosis (used by GetService).
	GetPodsForRelease(ctx context.Context, namespace, releaseID string) ([]PodInfo, error)

	// GetControllerReadiness returns true when all controller resources in the provided list
	// are ready. The implementation decides which kinds it handles; unknown kinds are ignored.
	GetControllerReadiness(ctx context.Context, namespace string, resources []ManifestResource) (bool, error)
}
