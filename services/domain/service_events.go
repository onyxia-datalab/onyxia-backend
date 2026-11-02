package domain

import "time"

// ServiceEventKind names an event of a service's stream.
type ServiceEventKind string

const (
	// ServiceEventStatus carries the service's state (Service).
	ServiceEventStatus ServiceEventKind = "status"
	// ServiceEventProgress carries a step reported by the cluster (Progress).
	ServiceEventProgress ServiceEventKind = "progress"
	// ServiceEventQuota carries the project's quota usage (Quota).
	ServiceEventQuota ServiceEventKind = "quota"
	// ServiceEventDone is the last event of the stream (End).
	ServiceEventDone ServiceEventKind = "done"
)

// ServiceEvent is an event of a service's stream. Only the field matching
// Kind is set.
type ServiceEvent struct {
	Kind     ServiceEventKind
	Service  *Service
	Progress *ServiceProgress
	Quota    *ProjectQuota
	End      StreamEndReason
}

// StreamEndReason tells why a service's stream ended.
type StreamEndReason string

const (
	// StreamEndStable: the service reached a stable status.
	StreamEndStable StreamEndReason = "stable"
	// StreamEndDeleted: the service no longer exists.
	StreamEndDeleted StreamEndReason = "deleted"
	// StreamEndTimeout: the stream reached its maximum duration while the
	// service was still changing.
	StreamEndTimeout StreamEndReason = "timeout"
)

// ServiceProgressStep is a step of a service reported by the cluster.
type ServiceProgressStep string

const (
	ServiceProgressScheduled        ServiceProgressStep = "scheduled"
	ServiceProgressSchedulingFailed ServiceProgressStep = "scheduling_failed"
	ServiceProgressPullingImage     ServiceProgressStep = "pulling_image"
	ServiceProgressImagePulled      ServiceProgressStep = "image_pulled"
	ServiceProgressStarted          ServiceProgressStep = "started"
	ServiceProgressProbeFailed      ServiceProgressStep = "probe_failed"
	ServiceProgressCreationFailed   ServiceProgressStep = "creation_failed"
	ServiceProgressOther            ServiceProgressStep = "other"
)

// ProgressLevel tells whether a step is expected or a problem.
type ProgressLevel string

const (
	ProgressLevelInfo    ProgressLevel = "info"
	ProgressLevelWarning ProgressLevel = "warning"
)

// ServiceProgress is a step of a service reported by the cluster, to show
// what is going on while its status stays the same.
type ServiceProgress struct {
	Step    ServiceProgressStep
	Level   ProgressLevel
	PodName string // empty when the step is not about a pod
	Image   string // set for the image steps
	Message string
	At      time.Time
}

// ProjectQuota is the resource quota of a project and its usage. Resources
// is empty when the project has no quota.
type ProjectQuota struct {
	Resources []QuotaResource
}

// QuotaResource is the limit and usage of one resource. Name is the resource
// as configured by the administrator (e.g. "requests.memory"); Hard and Used
// are quantities in the cluster's notation (e.g. "10Gi").
type QuotaResource struct {
	Name string
	Hard string
	Used string
}
