package query

import (
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/stretchr/testify/assert"
)

// --- deriveStatusFromHelm ---------------------------------------------------
// Tested via ListServices → buildLightService → deriveStatusLight → deriveStatusFromHelm
// (only reached when Helm status != "deployed").

func TestDeriveStatusFromHelm_Ghost(t *testing.T) {
	// Ghost is also handled explicitly in deriveStatusWithDetail, but
	// deriveStatusFromHelm covers it for the list path.
	svc, _ := readerForHelmStatus(t, ports.ReleaseState{Exists: false})
	assert.Equal(t, domain.ServiceStatusGhost, svc.Status)
}

func TestDeriveStatusFromHelm_PendingInstall(t *testing.T) {
	svc, _ := readerForHelmStatus(t, ports.ReleaseState{Exists: true, Status: "pending-install"})
	assert.Equal(t, domain.ServiceStatusDeploying, svc.Status)
}

func TestDeriveStatusFromHelm_PendingUpgrade(t *testing.T) {
	svc, _ := readerForHelmStatus(t, ports.ReleaseState{Exists: true, Status: "pending-upgrade"})
	assert.Equal(t, domain.ServiceStatusDeploying, svc.Status)
}

func TestDeriveStatusFromHelm_Failed(t *testing.T) {
	svc, _ := readerForHelmStatus(t, ports.ReleaseState{Exists: true, Status: "failed"})
	assert.Equal(t, domain.ServiceStatusError, svc.Status)
}

func TestDeriveStatusFromHelm_Uninstalling(t *testing.T) {
	svc, _ := readerForHelmStatus(t, ports.ReleaseState{Exists: true, Status: "uninstalling"})
	assert.Equal(t, domain.ServiceStatusTerminating, svc.Status)
}

func TestDeriveStatusFromHelm_Suspended(t *testing.T) {
	svc, _ := readerForHelmStatus(t, ports.ReleaseState{Exists: true, Suspended: true, Status: "pending-install"})
	assert.Equal(t, domain.ServiceStatusSuspended, svc.Status)
}

func TestDeriveStatusFromHelm_Superseded(t *testing.T) {
	svc, _ := readerForHelmStatus(t, ports.ReleaseState{Exists: true, Status: "superseded"})
	assert.Equal(t, domain.ServiceStatusRunning, svc.Status)
}

// Suspended is handled explicitly in deriveStatusWithDetail (GetService path),
// not through deriveStatusFromHelm.
func TestGetService_Suspended(t *testing.T) {
	svc, _ := readerForState(t, ports.ReleaseState{Exists: true, Suspended: true, Status: "deployed"})
	assert.Equal(t, domain.ServiceStatusSuspended, svc.Status)
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
		ErrorReason:  ports.PodErrorReasonCrashLoop,
		RestartCount: 5,
		Message:      "back-off restarting failed container",
	}}
	svc, _ := readerForPodsGetService(t, pods)
	assert.Equal(t, domain.ServiceStatusError, svc.Status)
	assert.NotNil(t, svc.Error)
	assert.Equal(t, domain.ServiceErrorReason(ports.PodErrorReasonCrashLoop), svc.Error.Reason)
	assert.Equal(t, int32(5), svc.Error.RestartCount)
}

func TestDerivePodStatus_OOMKilled(t *testing.T) {
	pods := []ports.PodInfo{{
		Name:        "pod-1",
		ErrorReason: ports.PodErrorReasonOOMKilled,
		ExitCode:    137,
	}}
	svc, _ := readerForPodsGetService(t, pods)
	assert.Equal(t, domain.ServiceStatusError, svc.Status)
	assert.Equal(t, domain.ServiceErrorReason(ports.PodErrorReasonOOMKilled), svc.Error.Reason)
	assert.Equal(t, int32(137), svc.Error.ExitCode)
}
