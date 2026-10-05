package query

import (
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

// statusFromRelease returns the status when the release state alone decides
// it. decided is false for an existing, non-suspended, deployed release: its
// status then depends on the workloads, which GetService and ListServices
// inspect at different levels of detail. Both paths go through this function
// so they never disagree on a status the release already decides.
func statusFromRelease(releaseState ports.ReleaseState) (status domain.ServiceStatus, decided bool) {
	if !releaseState.Exists {
		return domain.ServiceStatusGhost, true
	}
	// A suspended release is still deployed (suspension is an upgrade), so
	// this must be checked before looking at the status.
	if releaseState.Suspended {
		return domain.ServiceStatusSuspended, true
	}
	switch releaseState.Status {
	case ports.ReleaseStatusDeployed:
		return "", false
	case ports.ReleaseStatusFailed:
		return domain.ServiceStatusError, true
	case ports.ReleaseStatusUninstalling:
		return domain.ServiceStatusTerminating, true
	default: // pending, unknown
		return domain.ServiceStatusDeploying, true
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
				Reason:       pod.ErrorReason,
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
