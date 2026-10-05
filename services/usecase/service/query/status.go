package query

import (
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

// statusFromHelm returns the status when the Helm release state alone decides
// it. decided is false for an existing, non-suspended "deployed" or
// "superseded" release: its status then depends on the workloads, which
// GetService and ListServices inspect at different levels of detail.
// Both paths go through this function so they never disagree on a status
// Helm already knows.
func statusFromHelm(releaseState ports.ReleaseState) (status domain.ServiceStatus, decided bool) {
	if !releaseState.Exists {
		return domain.ServiceStatusGhost, true
	}
	// A suspended release stays "deployed" in Helm (suspension is a helm
	// upgrade), so this must be checked before looking at the status.
	if releaseState.Suspended {
		return domain.ServiceStatusSuspended, true
	}
	switch releaseState.Status {
	case "pending-install", "pending-upgrade", "pending-rollback", "unknown":
		return domain.ServiceStatusDeploying, true
	case "failed":
		return domain.ServiceStatusError, true
	case "uninstalling":
		return domain.ServiceStatusTerminating, true
	default: // "deployed", "superseded"
		return "", false
	}
}

// derivePodStatus maps pod states to a ServiceStatus.
// The first pod with an error determines the ServiceError detail.
func derivePodStatus(pods []ports.PodInfo) (domain.ServiceStatus, *domain.ServiceError) {
	if len(pods) == 0 {
		return domain.ServiceStatusDeploying, nil
	}

	allReady := true
	for _, pod := range pods {
		if !pod.Ready {
			allReady = false
		}
		if pod.ErrorReason != "" {
			return domain.ServiceStatusError, &domain.ServiceError{
				Reason:       domain.ServiceErrorReason(pod.ErrorReason),
				PodName:      pod.Name,
				Message:      pod.Message,
				RestartCount: pod.RestartCount,
				ExitCode:     pod.ExitCode,
				Image:        pod.Image,
				Limit:        pod.Limit,
			}
		}
	}

	if allReady {
		return domain.ServiceStatusRunning, nil
	}
	return domain.ServiceStatusDeploying, nil
}
