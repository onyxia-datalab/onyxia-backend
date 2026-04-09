package service

import (
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

// deriveStatusFromHelm maps a Helm release state to a ServiceStatus without querying pods.
// Used by ListServices where pod queries would be too expensive.
func deriveStatusFromHelm(releaseState ports.ReleaseState) domain.ServiceStatus {
	if !releaseState.Exists {
		return domain.ServiceStatusGhost
	}
	if releaseState.Suspended {
		return domain.ServiceStatusSuspended
	}
	switch releaseState.Status {
	case "pending-install", "pending-upgrade", "pending-rollback", "unknown":
		return domain.ServiceStatusDeploying
	case "failed":
		return domain.ServiceStatusError
	case "uninstalling":
		return domain.ServiceStatusTerminating
	default: // "deployed", "superseded"
		return domain.ServiceStatusRunning
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
