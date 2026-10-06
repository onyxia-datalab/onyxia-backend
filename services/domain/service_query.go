package domain

// ServiceStatus is the observed lifecycle state of a service.
type ServiceStatus string

const (
	ServiceStatusDeploying   ServiceStatus = "Deploying"
	ServiceStatusRunning     ServiceStatus = "Running"
	ServiceStatusError       ServiceStatus = "Error"
	ServiceStatusGhost       ServiceStatus = "Ghost"
	ServiceStatusSuspending  ServiceStatus = "Suspending"
	ServiceStatusSuspended   ServiceStatus = "Suspended"
	ServiceStatusTerminating ServiceStatus = "Terminating"
)

// ServiceErrorReason identifies the root cause of an Error state.
type ServiceErrorReason string

const (
	// ServiceErrorReasonReleaseFailed: the last install or upgrade of the
	// release failed. The other reasons come from a failing pod.
	ServiceErrorReasonReleaseFailed   ServiceErrorReason = "release_failed"
	ServiceErrorReasonCrashLoop       ServiceErrorReason = "crash_loop"
	ServiceErrorReasonOOMKilled       ServiceErrorReason = "oom_killed"
	ServiceErrorReasonImagePull       ServiceErrorReason = "image_pull"
	ServiceErrorReasonConfigError     ServiceErrorReason = "config_error"
	ServiceErrorReasonUnschedulable   ServiceErrorReason = "unschedulable"
	ServiceErrorReasonReadinessFailed ServiceErrorReason = "readiness_failed"
)

// ServiceError carries the detail of an Error state. PodName is empty when
// the cause is not a pod (ServiceErrorReasonReleaseFailed).
type ServiceError struct {
	Reason       ServiceErrorReason
	PodName      string
	Message      string
	RestartCount int32
	ExitCode     int32
	Image        string
	Limit        string
}

// Service is the read model returned by the ServiceQuery use case.
type Service struct {
	ReleaseID    string
	Namespace    string
	FriendlyName string
	Owner        string
	CatalogID    string
	Share        bool
	Status       ServiceStatus
	Error        *ServiceError
}
