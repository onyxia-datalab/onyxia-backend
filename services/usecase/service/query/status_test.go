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
		name       string
		release    ports.ReleaseState
		pods       []ports.PodInfo
		wantStatus domain.ServiceStatus
		wantError  *domain.ServiceError
	}{
		{"ghost", ports.ReleaseState{Exists: false}, nil, domain.ServiceStatusGhost, nil},
		{"pending-install", ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusPending}, nil, domain.ServiceStatusDeploying, nil},
		{"unknown", ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusUnknown}, nil, domain.ServiceStatusDeploying, nil},
		{
			"failed release carries the deployment tool's message",
			ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusFailed, Message: "timed out"},
			nil,
			domain.ServiceStatusError,
			&domain.ServiceError{Reason: domain.ServiceErrorReasonReleaseFailed, Message: "timed out"},
		},
		{"uninstalling", ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusUninstalling}, nil, domain.ServiceStatusTerminating, nil},
		{"suspended, no pod left", suspended, nil, domain.ServiceStatusSuspended, nil},
		{"suspended, pods shutting down", suspended, []ports.PodInfo{{Name: "p", Terminating: true}}, domain.ServiceStatusSuspending, nil},
		{"suspend in progress", ports.ReleaseState{Exists: true, Suspended: true, Status: ports.ReleaseStatusPending}, []ports.PodInfo{ready}, domain.ServiceStatusSuspending, nil},
		{"deployed, no pod yet", deployed, nil, domain.ServiceStatusDeploying, nil},
		{"deployed, all pods ready", deployed, []ports.PodInfo{ready}, domain.ServiceStatusRunning, nil},
		{"deployed, a pod not ready", deployed, []ports.PodInfo{ready, {Name: "starting"}}, domain.ServiceStatusDeploying, nil},
		{
			"deployed, a pod failing",
			deployed,
			[]ports.PodInfo{ready, crashing},
			domain.ServiceStatusError,
			&domain.ServiceError{Reason: domain.ServiceErrorReasonCrashLoop, PodName: "crash"},
		},
		{
			"rollout: the old pod shutting down doesn't count",
			deployed,
			[]ports.PodInfo{ready, {Name: "old", Terminating: true}},
			domain.ServiceStatusRunning,
			nil,
		},
		{
			"rollout: only the pod shutting down is left",
			deployed,
			[]ports.PodInfo{{Name: "old", Ready: true, Terminating: true}},
			domain.ServiceStatusDeploying,
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, svcErr := deriveStatus(tt.release, tt.pods)
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantError, svcErr)
			// The detail is present exactly when the status is Error.
			assert.Equal(t, status == domain.ServiceStatusError, svcErr != nil)
		})
	}
}

func TestNeedsPods(t *testing.T) {
	assert.False(t, needsPods(ports.ReleaseState{Exists: false}))
	assert.False(t, needsPods(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusFailed}))
	assert.False(t, needsPods(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusPending}))
	assert.True(t, needsPods(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}))
	assert.True(t, needsPods(ports.ReleaseState{Exists: true, Suspended: true, Status: ports.ReleaseStatusPending}))
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
