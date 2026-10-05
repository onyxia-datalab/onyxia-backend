package domain

// ReleaseEvent is emitted on the SSE stream while watching an install.
type ReleaseEvent struct {
	Kind    ReleaseEventKind
	Status  ReleaseEventStatus
	Name    string // release name or pod name
	Message string // optional human-readable detail
}

// ReleaseEventKind distinguishes the source of the event.
type ReleaseEventKind string

const (
	ReleaseEventKindHelm ReleaseEventKind = "helm"
	ReleaseEventKindPod  ReleaseEventKind = "pod"
)

// ReleaseEventStatus is the observed status.
type ReleaseEventStatus string

const (
	// Helm statuses
	ReleaseStatusDeploying ReleaseEventStatus = "deploying" // pending-install
	ReleaseStatusDeployed  ReleaseEventStatus = "deployed"  // terminal success
	ReleaseStatusFailed    ReleaseEventStatus = "failed"    // terminal failure

	// Pod statuses
	PodStatusPending ReleaseEventStatus = "pending" // scheduled, containers not started
	PodStatusRunning ReleaseEventStatus = "running" // at least one container running
	PodStatusReady   ReleaseEventStatus = "ready"   // all containers ready
	PodStatusFailed  ReleaseEventStatus = "failed"  // at least one container failed
)
