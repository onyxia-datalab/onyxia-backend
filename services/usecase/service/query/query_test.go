package query

import (
	"context"
	"errors"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/namespace"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// queryMocks groups all dependencies needed to build a Reader.
type queryMocks struct {
	helm    *mocks.MockReleaseGateway
	records *mocks.MockServiceRecordGateway
	pods    *mocks.MockWorkloadStateGateway
}

// setupReader creates a Reader with a context that carries the given username.
func setupReader(t *testing.T, username string) (*Reader, context.Context, queryMocks) {
	t.Helper()
	m := queryMocks{
		helm:    new(mocks.MockReleaseGateway),
		records: new(mocks.MockServiceRecordGateway),
		pods:    new(mocks.MockWorkloadStateGateway),
	}
	ctx, reader, _ := usercontext.NewTestUserContext(&usercontext.User{Username: username})
	uc := NewReader(m.records, m.helm, m.pods, reader, namespace.NewAuthorizer("user-", "projet-"))
	return uc, ctx, m
}

const (
	// A group namespace: the owner/share filter only applies there.
	testNamespace     = "projet-data-team"
	personalNamespace = "user-alice"
	testRelease       = "jupyter-abc"
	testUsername      = "alice"
)

func record(releaseID, owner string, share bool) ports.ServiceRecord {
	return ports.ServiceRecord{
		ReleaseID:    releaseID,
		FriendlyName: "My Service",
		Owner:        owner,
		CatalogID:    "my-catalog",
		Share:        share,
	}
}

// readerForState drives GetService through the Helm-state path.
// Covers Ghost (Exists=false) and Suspended (both decided by statusFromRelease).
func readerForState(t *testing.T, state ports.ReleaseState) (domain.Service, error) {
	t.Helper()
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, testUsername, false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(state, nil)

	if state.Exists && !state.Suspended {
		m.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
			Return([]ports.PodInfo{}, nil)
	}

	return uc.GetService(ctx, testNamespace, testRelease)
}

// readerForHelmStatus drives ListServices for a release whose status Helm
// alone decides (see statusFromRelease): no workload mock is set up.
func readerForHelmStatus(t *testing.T, state ports.ReleaseState) (domain.Service, error) {
	t.Helper()
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("ListServiceRecords", mock.Anything, testNamespace).
		Return([]ports.ServiceRecord{record(testRelease, testUsername, false)}, nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(state, nil)

	svcs, err := uc.ListServices(ctx, testNamespace)
	if err != nil || len(svcs) == 0 {
		return domain.Service{}, err
	}
	return svcs[0], nil
}

// readerForPodsGetService drives GetService through the pod-status path.
// Used by status_test.go.
func readerForPodsGetService(t *testing.T, pods []ports.PodInfo) (domain.Service, error) {
	t.Helper()
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, testUsername, false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
		Return(pods, nil)

	return uc.GetService(ctx, testNamespace, testRelease)
}

// --- GetService -------------------------------------------------------------

func TestGetService_Ghost(t *testing.T) {
	svc, err := readerForState(t, ports.ReleaseState{Exists: false})
	require.NoError(t, err)
	assert.Equal(t, domain.ServiceStatusGhost, svc.Status)
}

func TestGetService_RecordNotFound(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(ports.ServiceRecord{}, domain.ErrNotFound)

	_, err := uc.GetService(ctx, testNamespace, testRelease)

	assert.ErrorIs(t, err, domain.ErrNotFound)
	m.helm.AssertNotCalled(t, "GetReleaseState")
}

func TestGetService_RecordReadError(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(ports.ServiceRecord{}, errors.New("k8s unavailable"))

	_, err := uc.GetService(ctx, testNamespace, testRelease)

	assert.ErrorContains(t, err, "k8s unavailable")
}

func TestGetService_DeniesUnownedUnshared(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, "someone-else", false), nil)

	_, err := uc.GetService(ctx, testNamespace, testRelease)

	assert.ErrorIs(t, err, domain.ErrNotFound)
	m.helm.AssertNotCalled(t, "GetReleaseState")
}

func TestGetService_AllowsSharedFromOtherOwner(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, "someone-else", true), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
		Return([]ports.PodInfo{}, nil)

	svc, err := uc.GetService(ctx, testNamespace, testRelease)

	require.NoError(t, err)
	assert.Equal(t, "someone-else", svc.Owner)
	assert.True(t, svc.Share)
}

func TestGetService_AllowsOwnerCaseInsensitive(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, "ALICE", false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
		Return([]ports.PodInfo{}, nil)

	svc, err := uc.GetService(ctx, testNamespace, testRelease)

	require.NoError(t, err)
	assert.Equal(t, "ALICE", svc.Owner)
}

func TestGetService_PersonalNamespaceShowsAnyOwner(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("GetServiceRecord", mock.Anything, personalNamespace, testRelease).
		Return(record(testRelease, "someone-else", false), nil)
	m.helm.On("GetReleaseState", mock.Anything, personalNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetPodsForRelease", mock.Anything, personalNamespace, testRelease).
		Return([]ports.PodInfo{}, nil)

	svc, err := uc.GetService(ctx, personalNamespace, testRelease)

	require.NoError(t, err)
	assert.Equal(t, "someone-else", svc.Owner)
}

func TestGetService_FieldsMappedFromRecord(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, testUsername, true), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
		Return([]ports.PodInfo{{Name: "p", Ready: true}}, nil)

	svc, err := uc.GetService(ctx, testNamespace, testRelease)

	require.NoError(t, err)
	assert.Equal(t, testRelease, svc.ReleaseID)
	assert.Equal(t, testNamespace, svc.Namespace)
	assert.Equal(t, "My Service", svc.FriendlyName)
	assert.Equal(t, testUsername, svc.Owner)
	assert.Equal(t, "my-catalog", svc.CatalogID)
	assert.True(t, svc.Share)
}

// --- ListServices -----------------------------------------------------------

func TestListServices_Empty(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("ListServiceRecords", mock.Anything, testNamespace).
		Return([]ports.ServiceRecord{}, nil)

	svcs, err := uc.ListServices(ctx, testNamespace)

	require.NoError(t, err)
	assert.Empty(t, svcs)
}

func TestListServices_ListError(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("ListServiceRecords", mock.Anything, testNamespace).
		Return(nil, errors.New("api error"))

	_, err := uc.ListServices(ctx, testNamespace)

	assert.ErrorContains(t, err, "api error")
}

func TestListServices_FiltersOutOtherOwnerUnshared(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("ListServiceRecords", mock.Anything, testNamespace).
		Return([]ports.ServiceRecord{record("svc-bob", "bob", false)}, nil)

	svcs, err := uc.ListServices(ctx, testNamespace)

	require.NoError(t, err)
	assert.Empty(t, svcs)
	// The state of a service the caller can't see is not even looked up.
	m.helm.AssertNotCalled(t, "GetReleaseState", mock.Anything, mock.Anything, mock.Anything)
}

func TestListServices_IncludesOwnedService(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("ListServiceRecords", mock.Anything, testNamespace).
		Return([]ports.ServiceRecord{record(testRelease, testUsername, false)}, nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.helm.On("GetReleaseResources", mock.Anything, testNamespace, testRelease).
		Return([]ports.ManifestResource{}, nil)
	m.pods.On("GetControllerReadiness", mock.Anything, testNamespace, mock.Anything).
		Return(true, nil)

	svcs, err := uc.ListServices(ctx, testNamespace)

	require.NoError(t, err)
	require.Len(t, svcs, 1)
	assert.Equal(t, testRelease, svcs[0].ReleaseID)
}

func TestListServices_IncludesSharedServiceFromOtherOwner(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("ListServiceRecords", mock.Anything, testNamespace).
		Return([]ports.ServiceRecord{record("svc-bob-shared", "bob", true)}, nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, "svc-bob-shared").
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.helm.On("GetReleaseResources", mock.Anything, testNamespace, "svc-bob-shared").
		Return([]ports.ManifestResource{}, nil)
	m.pods.On("GetControllerReadiness", mock.Anything, testNamespace, mock.Anything).
		Return(true, nil)

	svcs, err := uc.ListServices(ctx, testNamespace)

	require.NoError(t, err)
	require.Len(t, svcs, 1)
	assert.Equal(t, "svc-bob-shared", svcs[0].ReleaseID)
}

func TestListServices_PersonalNamespaceShowsAnyOwner(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("ListServiceRecords", mock.Anything, personalNamespace).
		Return([]ports.ServiceRecord{record("svc-legacy", "legacy-owner", false)}, nil)
	m.helm.On("GetReleaseState", mock.Anything, personalNamespace, "svc-legacy").
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.helm.On("GetReleaseResources", mock.Anything, personalNamespace, "svc-legacy").
		Return([]ports.ManifestResource{}, nil)
	m.pods.On("GetControllerReadiness", mock.Anything, personalNamespace, mock.Anything).
		Return(true, nil)

	svcs, err := uc.ListServices(ctx, personalNamespace)

	require.NoError(t, err)
	require.Len(t, svcs, 1)
	assert.Equal(t, "svc-legacy", svcs[0].ReleaseID)
}

// --- deriveStatusLight (via ListServices) -----------------------------------

func TestListServices_DeployedRelease_Ready(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	resources := []ports.ManifestResource{{Kind: "Deployment", Name: "my-deploy"}}

	m.records.On("ListServiceRecords", mock.Anything, testNamespace).
		Return([]ports.ServiceRecord{record(testRelease, testUsername, false)}, nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.helm.On("GetReleaseResources", mock.Anything, testNamespace, testRelease).
		Return(resources, nil)
	m.pods.On("GetControllerReadiness", mock.Anything, testNamespace, resources).
		Return(true, nil)

	svcs, err := uc.ListServices(ctx, testNamespace)

	require.NoError(t, err)
	require.Len(t, svcs, 1)
	assert.Equal(t, domain.ServiceStatusRunning, svcs[0].Status)
}

func TestListServices_DeployedRelease_NotReady(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	resources := []ports.ManifestResource{{Kind: "Deployment", Name: "my-deploy"}}

	m.records.On("ListServiceRecords", mock.Anything, testNamespace).
		Return([]ports.ServiceRecord{record(testRelease, testUsername, false)}, nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.helm.On("GetReleaseResources", mock.Anything, testNamespace, testRelease).
		Return(resources, nil)
	m.pods.On("GetControllerReadiness", mock.Anything, testNamespace, resources).
		Return(false, nil)

	svcs, err := uc.ListServices(ctx, testNamespace)

	require.NoError(t, err)
	require.Len(t, svcs, 1)
	assert.Equal(t, domain.ServiceStatusDeploying, svcs[0].Status)
}

func TestGetService_HelmStateError(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, testUsername, false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{}, errors.New("helm down"))

	_, err := uc.GetService(ctx, testNamespace, testRelease)

	assert.ErrorContains(t, err, "helm down")
}

func TestGetService_PodQueryError(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, testUsername, false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
		Return(nil, errors.New("k8s down"))

	_, err := uc.GetService(ctx, testNamespace, testRelease)

	assert.ErrorContains(t, err, "k8s down")
}

func TestListServices_HelmStateError(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("ListServiceRecords", mock.Anything, testNamespace).
		Return([]ports.ServiceRecord{record(testRelease, testUsername, false)}, nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{}, errors.New("helm down"))

	_, err := uc.ListServices(ctx, testNamespace)

	assert.ErrorContains(t, err, "helm down")
}

func TestListServices_ReleaseResourcesError(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("ListServiceRecords", mock.Anything, testNamespace).
		Return([]ports.ServiceRecord{record(testRelease, testUsername, false)}, nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.helm.On("GetReleaseResources", mock.Anything, testNamespace, testRelease).
		Return(nil, errors.New("k8s down"))

	_, err := uc.ListServices(ctx, testNamespace)

	assert.ErrorContains(t, err, "k8s down")
}

func TestListServices_ControllerReadinessError(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("ListServiceRecords", mock.Anything, testNamespace).
		Return([]ports.ServiceRecord{record(testRelease, testUsername, false)}, nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.helm.On("GetReleaseResources", mock.Anything, testNamespace, testRelease).
		Return([]ports.ManifestResource{}, nil)
	m.pods.On("GetControllerReadiness", mock.Anything, testNamespace, []ports.ManifestResource{}).
		Return(false, errors.New("k8s down"))

	_, err := uc.ListServices(ctx, testNamespace)

	assert.ErrorContains(t, err, "k8s down")
}

func TestListServices_NonDeployedRelease_NoK8sCall(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.records.On("ListServiceRecords", mock.Anything, testNamespace).
		Return([]ports.ServiceRecord{record(testRelease, testUsername, false)}, nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusPending}, nil)

	svcs, err := uc.ListServices(ctx, testNamespace)

	require.NoError(t, err)
	require.Len(t, svcs, 1)
	assert.Equal(t, domain.ServiceStatusDeploying, svcs[0].Status)
	m.pods.AssertNotCalled(t, "GetControllerReadiness")
	m.helm.AssertNotCalled(t, "GetReleaseResources")
}
