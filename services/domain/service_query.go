package domain

import "context"

// ServiceStatus is the observed lifecycle state of a service.
type ServiceStatus string

const (
	ServiceStatusDeploying   ServiceStatus = "Deploying"
	ServiceStatusRunning     ServiceStatus = "Running"
	ServiceStatusError       ServiceStatus = "Error"
	ServiceStatusGhost       ServiceStatus = "Ghost"
	ServiceStatusSuspended   ServiceStatus = "Suspended"
	ServiceStatusTerminating ServiceStatus = "Terminating"
)

// ServiceErrorReason identifies the root cause of an Erreur state.
type ServiceErrorReason string

const (
	ServiceErrorReasonCrashLoop       ServiceErrorReason = "crash_loop"
	ServiceErrorReasonOOMKilled       ServiceErrorReason = "oom_killed"
	ServiceErrorReasonImagePull       ServiceErrorReason = "image_pull"
	ServiceErrorReasonConfigError     ServiceErrorReason = "config_error"
	ServiceErrorReasonUnschedulable   ServiceErrorReason = "unschedulable"
	ServiceErrorReasonReadinessFailed ServiceErrorReason = "readiness_failed"
)

// ServiceError carries the detail of an Erreur state.
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

// ServiceQuery is the read side of the service lifecycle.
type ServiceQuery interface {
	GetService(ctx context.Context, namespace, releaseID string) (Service, error)
	ListServices(ctx context.Context, namespace string) ([]Service, error)
}
