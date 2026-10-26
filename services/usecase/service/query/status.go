package query

import (
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

// workloads is what the cluster reports about the workloads of a release.
type workloads struct {
	// pods are the release's live pods.
	pods []ports.PodInfo
	// quotaFailure is why the release's pods can't be created because the
	// quota is exceeded; empty when they can.
	quotaFailure string
}

// needsWorkloads reports whether deriveStatus needs the release's workloads:
// a pending, failed or uninstalling release decides alone. Callers skip the
// workload queries otherwise.
func needsWorkloads(release ports.ReleaseState) bool {
	return !release.Exists || release.Suspended || release.Status == ports.ReleaseStatusDeployed
}

// deriveStatus is the single derivation of a service's status: GetService,
// ListServices and the event stream all go through it, so they can't
// disagree. w is only read when needsWorkloads(release).
// The ServiceError is non-nil exactly when the status is Error.
func deriveStatus(release ports.ReleaseState, w workloads) (domain.ServiceStatus, *domain.ServiceError) {
	if !release.Exists {
		// Uninstalling deletes the release first: its pods then shut down.
		if len(w.pods) > 0 {
			return domain.ServiceStatusTerminating, nil
		}
		return domain.ServiceStatusGhost, nil
	}
	// A suspended release is still deployed (suspension is an upgrade), so
	// this must be checked before looking at the status.
	if release.Suspended {
		if len(w.pods) > 0 {
			return domain.ServiceStatusSuspending, nil
		}
		return domain.ServiceStatusSuspended, nil
	}
	switch release.Status {
	case ports.ReleaseStatusDeployed:
		status, svcErr := derivePodStatus(activePods(w.pods))
		if status == domain.ServiceStatusDeploying && w.quotaFailure != "" {
			return domain.ServiceStatusError, &domain.ServiceError{
				Reason:  domain.ServiceErrorReasonQuotaExceeded,
				Message: w.quotaFailure,
			}
		}
		return status, svcErr
	case ports.ReleaseStatusFailed:
		return domain.ServiceStatusError, &domain.ServiceError{
			Reason:  domain.ServiceErrorReasonReleaseFailed,
			Message: release.Message,
		}
	case ports.ReleaseStatusUninstalling:
		return domain.ServiceStatusTerminating, nil
	default: // pending, unknown
		return domain.ServiceStatusDeploying, nil
	}
}

// activePods leaves out the pods being deleted: during a rollout the old pod
// shuts down while its replacement starts, and only the latter tells whether
// the service is up.
func activePods(pods []ports.PodInfo) []ports.PodInfo {
	active := make([]ports.PodInfo, 0, len(pods))
	for _, pod := range pods {
		if !pod.Terminating {
			active = append(active, pod)
		}
	}
	return active
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
