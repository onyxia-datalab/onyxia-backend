package query

import (
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- deriveStatus -----------------------------------------------------------

func TestDeriveStatus(t *testing.T) {
	deployed := ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}
	suspended := ports.ReleaseState{Exists: true, Suspended: true, Status: ports.ReleaseStatusDeployed}
	ready := ports.PodInfo{Name: "new", Ready: true}
	crashing := ports.PodInfo{Name: "crash", ErrorReason: domain.ServiceErrorReasonCrashLoop}

	tests := []struct {
		name         string
		release      ports.ReleaseState
		pods         []ports.PodInfo
		quotaFailure string
		wantStatus   domain.ServiceStatus
		wantError    *domain.ServiceError
	}{
		{name: "ghost", release: ports.ReleaseState{}, wantStatus: domain.ServiceStatusGhost},
		{
			name:       "release uninstalled, pods shutting down",
			release:    ports.ReleaseState{},
			pods:       []ports.PodInfo{{Name: "p", Terminating: true}},
			wantStatus: domain.ServiceStatusTerminating,
		},
		{name: "pending-install", release: ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusPending}, wantStatus: domain.ServiceStatusDeploying},
		{name: "unknown", release: ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusUnknown}, wantStatus: domain.ServiceStatusDeploying},
		{
			name:       "failed release carries the deployment tool's message",
			release:    ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusFailed, Message: "timed out"},
			wantStatus: domain.ServiceStatusError,
			wantError:  &domain.ServiceError{Reason: domain.ServiceErrorReasonReleaseFailed, Message: "timed out"},
		},
		{name: "uninstalling", release: ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusUninstalling}, wantStatus: domain.ServiceStatusTerminating},
		{name: "suspended, no pod left", release: suspended, wantStatus: domain.ServiceStatusSuspended},
		{
			name:       "suspended, pods shutting down",
			release:    suspended,
			pods:       []ports.PodInfo{{Name: "p", Terminating: true}},
			wantStatus: domain.ServiceStatusSuspending,
		},
		{
			name:       "suspend in progress",
			release:    ports.ReleaseState{Exists: true, Suspended: true, Status: ports.ReleaseStatusPending},
			pods:       []ports.PodInfo{ready},
			wantStatus: domain.ServiceStatusSuspending,
		},
		{name: "deployed, no pod yet", release: deployed, wantStatus: domain.ServiceStatusDeploying},
		{name: "deployed, all pods ready", release: deployed, pods: []ports.PodInfo{ready}, wantStatus: domain.ServiceStatusRunning},
		{
			name:       "deployed, a pod not ready",
			release:    deployed,
			pods:       []ports.PodInfo{ready, {Name: "starting"}},
			wantStatus: domain.ServiceStatusDeploying,
		},
		{
			name:       "deployed, a pod failing",
			release:    deployed,
			pods:       []ports.PodInfo{ready, crashing},
			wantStatus: domain.ServiceStatusError,
			wantError:  &domain.ServiceError{Reason: domain.ServiceErrorReasonCrashLoop, PodName: "crash"},
		},
		{
			name:       "rollout: the old pod shutting down doesn't count",
			release:    deployed,
			pods:       []ports.PodInfo{ready, {Name: "old", Terminating: true}},
			wantStatus: domain.ServiceStatusRunning,
		},
		{
			name:       "rollout: only the pod shutting down is left",
			release:    deployed,
			pods:       []ports.PodInfo{{Name: "old", Ready: true, Terminating: true}},
			wantStatus: domain.ServiceStatusDeploying,
		},
		{
			name:         "pods refused by the quota",
			release:      deployed,
			quotaFailure: "exceeded quota: requests.memory",
			wantStatus:   domain.ServiceStatusError,
			wantError: &domain.ServiceError{
				Reason:  domain.ServiceErrorReasonQuotaExceeded,
				Message: "exceeded quota: requests.memory",
			},
		},
		{
			name:         "a pod failing outranks the quota",
			release:      deployed,
			pods:         []ports.PodInfo{crashing},
			quotaFailure: "exceeded quota",
			wantStatus:   domain.ServiceStatusError,
			wantError:    &domain.ServiceError{Reason: domain.ServiceErrorReasonCrashLoop, PodName: "crash"},
		},
		{
			name:         "the quota doesn't matter once the pods are up",
			release:      deployed,
			pods:         []ports.PodInfo{ready},
			quotaFailure: "exceeded quota",
			wantStatus:   domain.ServiceStatusRunning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, svcErr := deriveStatus(tt.release, workloads{pods: tt.pods, quotaFailure: tt.quotaFailure})
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantError, svcErr)
			// The detail is present exactly when the status is Error.
			assert.Equal(t, status == domain.ServiceStatusError, svcErr != nil)
		})
	}
}

func TestNeedsWorkloads(t *testing.T) {
	assert.True(t, needsWorkloads(ports.ReleaseState{Exists: false}))
	assert.False(t, needsWorkloads(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusFailed}))
	assert.False(t, needsWorkloads(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusPending}))
	assert.True(t, needsWorkloads(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}))
	assert.True(t, needsWorkloads(ports.ReleaseState{Exists: true, Suspended: true, Status: ports.ReleaseStatusPending}))
}

func TestGetService_Suspended(t *testing.T) {
	svc, _ := readerForState(t, ports.ReleaseState{Exists: true, Suspended: true, Status: ports.ReleaseStatusDeployed})
	assert.Equal(t, domain.ServiceStatusSuspended, svc.Status)
}

// Statuses the release alone decides are derived without looking at pods.
func TestGetService_ReleaseDecidedStatuses(t *testing.T) {
	tests := []struct {
		releaseStatus ports.ReleaseStatus
		want          domain.ServiceStatus
	}{
		{ports.ReleaseStatusFailed, domain.ServiceStatusError},
		{ports.ReleaseStatusUninstalling, domain.ServiceStatusTerminating},
		{ports.ReleaseStatusPending, domain.ServiceStatusDeploying},
	}

	for _, tt := range tests {
		t.Run(string(tt.releaseStatus), func(t *testing.T) {
			uc, ctx, m := setupReader(t)
			m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
				Return(record(testRelease, testUsername, false), nil)
			m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
				Return(ports.ReleaseState{Exists: true, Status: tt.releaseStatus}, nil)

			svc, err := uc.GetService(ctx, testCaller, testNamespace, testRelease)

			assert.NoError(t, err)
			assert.Equal(t, tt.want, svc.Status)
			m.pods.AssertNotCalled(t, "GetPodsForRelease", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// --- derivePodStatus --------------------------------------------------------
// Tested via GetService → deriveStatus → derivePodStatus.

func TestDerivePodStatus_NoPods(t *testing.T) {
	svc, _ := readerForPodsGetService(t, []ports.PodInfo{})
	assert.Equal(t, domain.ServiceStatusDeploying, svc.Status)
	assert.Nil(t, svc.Error)
}

func TestDerivePodStatus_AllReady(t *testing.T) {
	pods := []ports.PodInfo{{Name: "pod-1", Ready: true}}
	svc, _ := readerForPodsGetService(t, pods)
	assert.Equal(t, domain.ServiceStatusRunning, svc.Status)
	assert.Nil(t, svc.Error)
}

func TestDerivePodStatus_NotAllReady(t *testing.T) {
	pods := []ports.PodInfo{{Name: "pod-1", Ready: false}}
	svc, _ := readerForPodsGetService(t, pods)
	assert.Equal(t, domain.ServiceStatusDeploying, svc.Status)
	assert.Nil(t, svc.Error)
}

func TestDerivePodStatus_CrashLoop(t *testing.T) {
	pods := []ports.PodInfo{{
		Name:         "pod-1",
		ErrorReason:  domain.ServiceErrorReasonCrashLoop,
		RestartCount: 5,
		Message:      "back-off restarting failed container",
	}}
	svc, _ := readerForPodsGetService(t, pods)
	assert.Equal(t, domain.ServiceStatusError, svc.Status)
	assert.NotNil(t, svc.Error)
	assert.Equal(t, domain.ServiceErrorReason(domain.ServiceErrorReasonCrashLoop), svc.Error.Reason)
	assert.Equal(t, int32(5), svc.Error.RestartCount)
}

func TestDerivePodStatus_OOMKilled(t *testing.T) {
	pods := []ports.PodInfo{{
		Name:        "pod-1",
		ErrorReason: domain.ServiceErrorReasonOOMKilled,
		ExitCode:    137,
	}}
	svc, _ := readerForPodsGetService(t, pods)
	assert.Equal(t, domain.ServiceStatusError, svc.Status)
	assert.Equal(t, domain.ServiceErrorReason(domain.ServiceErrorReasonOOMKilled), svc.Error.Reason)
	assert.Equal(t, int32(137), svc.Error.ExitCode)
}
