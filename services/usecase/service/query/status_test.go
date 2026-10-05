package query

import (
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- statusFromRelease ---------------------------------------------------------

func TestStatusFromHelm(t *testing.T) {
	tests := []struct {
		name        string
		state       ports.ReleaseState
		wantStatus  domain.ServiceStatus
		wantDecided bool
	}{
		{"ghost", ports.ReleaseState{Exists: false}, domain.ServiceStatusGhost, true},
		{"pending-install", ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusPending}, domain.ServiceStatusDeploying, true},
		{"unknown", ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusUnknown}, domain.ServiceStatusDeploying, true},
		{"failed", ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusFailed}, domain.ServiceStatusError, true},
		{"uninstalling", ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusUninstalling}, domain.ServiceStatusTerminating, true},
		{"suspended while pending", ports.ReleaseState{Exists: true, Suspended: true, Status: ports.ReleaseStatusPending}, domain.ServiceStatusSuspended, true},
		{"suspended while deployed", ports.ReleaseState{Exists: true, Suspended: true, Status: ports.ReleaseStatusDeployed}, domain.ServiceStatusSuspended, true},
		{"deployed depends on workloads", ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, decided := statusFromRelease(tt.state)
			assert.Equal(t, tt.wantDecided, decided)
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}

// A suspend is a helm upgrade, so a suspended release is "deployed". The list
// path used to only consult Helm for non-deployed releases and reported it
// as Running (its workloads, scaled to 0, are trivially "ready").
func TestListServices_SuspendedDeployedRelease(t *testing.T) {
	svc, err := readerForHelmStatus(t, ports.ReleaseState{Exists: true, Suspended: true, Status: ports.ReleaseStatusDeployed})
	assert.NoError(t, err)
	assert.Equal(t, domain.ServiceStatusSuspended, svc.Status)
}

func TestGetService_Suspended(t *testing.T) {
	svc, _ := readerForState(t, ports.ReleaseState{Exists: true, Suspended: true, Status: ports.ReleaseStatusDeployed})
	assert.Equal(t, domain.ServiceStatusSuspended, svc.Status)
}

// GetService must agree with ListServices on statuses the release already decides,
// without looking at pods.
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
			uc, ctx, m := setupReader(t, testUsername)
			m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
				Return(record(testRelease, testUsername, false), nil)
			m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
				Return(ports.ReleaseState{Exists: true, Status: tt.releaseStatus}, nil)

			svc, err := uc.GetService(ctx, testNamespace, testRelease)

			assert.NoError(t, err)
			assert.Equal(t, tt.want, svc.Status)
			m.pods.AssertNotCalled(t, "GetPodsForRelease")
		})
	}
}

// --- derivePodStatus --------------------------------------------------------
// Tested via GetService → deriveStatusWithDetail → derivePodStatus.

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
