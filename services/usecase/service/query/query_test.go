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

// setupReader creates a Reader and the mocks of its gateways.
func setupReader(t *testing.T) (*Reader, context.Context, queryMocks) {
	t.Helper()
	m := queryMocks{
		helm:    new(mocks.MockReleaseGateway),
		records: new(mocks.MockServiceRecordGateway),
		pods:    new(mocks.MockWorkloadStateGateway),
	}
	uc := NewReader(m.records, m.helm, m.pods, namespace.NewAuthorizer("user-", "projet-"))
	return uc, context.Background(), m
}

const (
	// A group namespace: the owner/share filter only applies there.
	testNamespace     = "projet-data-team"
	personalNamespace = "user-alice"
	testRelease       = "jupyter-abc"
	testUsername      = "alice"
)

// testCaller may act in both testNamespace (through its group) and
// personalNamespace.
var testCaller = usercontext.User{Username: testUsername, Groups: []string{"data-team"}}

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
	uc, ctx, m := setupReader(t)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, testUsername, false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(state, nil)

	if state.Exists && !state.Suspended {
		m.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
			Return([]ports.PodInfo{}, nil)
	}

	return uc.GetService(ctx, testCaller, testNamespace, testRelease)
}

// readerForHelmStatus drives ListServices for a release whose status Helm
// alone decides (see statusFromRelease): no workload mock is set up.
func readerForHelmStatus(t *testing.T, state ports.ReleaseState) (domain.Service, error) {
	t.Helper()
	uc, ctx, m := setupReader(t)

	m.records.On("ListServiceRecords", mock.Anything, testNamespace).
		Return([]ports.ServiceRecord{record(testRelease, testUsername, false)}, nil)
	m.helm.On("ListReleaseStates", mock.Anything, testNamespace).
		Return(map[string]ports.ReleaseState{testRelease: state}, nil)

	svcs, err := uc.ListServices(ctx, testCaller, testNamespace)
	if err != nil || len(svcs) == 0 {
		return domain.Service{}, err
	}
	return svcs[0], nil
}

// readerForPodsGetService drives GetService through the pod-status path.
// Used by status_test.go.
func readerForPodsGetService(t *testing.T, pods []ports.PodInfo) (domain.Service, error) {
	t.Helper()
	uc, ctx, m := setupReader(t)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, testUsername, false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
		Return(pods, nil)

	return uc.GetService(ctx, testCaller, testNamespace, testRelease)
}

// --- GetService -------------------------------------------------------------

func TestGetService_Ghost(t *testing.T) {
	svc, err := readerForState(t, ports.ReleaseState{Exists: false})
	require.NoError(t, err)
	assert.Equal(t, domain.ServiceStatusGhost, svc.Status)
}

func TestGetService_RecordNotFound(t *testing.T) {
	uc, ctx, m := setupReader(t)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(ports.ServiceRecord{}, domain.ErrNotFound)

	_, err := uc.GetService(ctx, testCaller, testNamespace, testRelease)

	assert.ErrorIs(t, err, domain.ErrNotFound)
	m.helm.AssertNotCalled(t, "GetReleaseState")
}

func TestGetService_RecordReadError(t *testing.T) {
	uc, ctx, m := setupReader(t)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(ports.ServiceRecord{}, errors.New("k8s unavailable"))

	_, err := uc.GetService(ctx, testCaller, testNamespace, testRelease)

	assert.ErrorContains(t, err, "k8s unavailable")
}

func TestGetService_DeniesUnownedUnshared(t *testing.T) {
	uc, ctx, m := setupReader(t)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, "someone-else", false), nil)

	_, err := uc.GetService(ctx, testCaller, testNamespace, testRelease)

	assert.ErrorIs(t, err, domain.ErrNotFound)
	m.helm.AssertNotCalled(t, "GetReleaseState")
}

func TestGetService_AllowsSharedFromOtherOwner(t *testing.T) {
	uc, ctx, m := setupReader(t)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, "someone-else", true), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
		Return([]ports.PodInfo{}, nil)

	svc, err := uc.GetService(ctx, testCaller, testNamespace, testRelease)

	require.NoError(t, err)
	assert.Equal(t, "someone-else", svc.Owner)
	assert.True(t, svc.Share)
}

func TestGetService_AllowsOwnerCaseInsensitive(t *testing.T) {
	uc, ctx, m := setupReader(t)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, "ALICE", false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
		Return([]ports.PodInfo{}, nil)

	svc, err := uc.GetService(ctx, testCaller, testNamespace, testRelease)

	require.NoError(t, err)
	assert.Equal(t, "ALICE", svc.Owner)
}

func TestGetService_PersonalNamespaceShowsAnyOwner(t *testing.T) {
	uc, ctx, m := setupReader(t)

	m.records.On("GetServiceRecord", mock.Anything, personalNamespace, testRelease).
		Return(record(testRelease, "someone-else", false), nil)
	m.helm.On("GetReleaseState", mock.Anything, personalNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetPodsForRelease", mock.Anything, personalNamespace, testRelease).
		Return([]ports.PodInfo{}, nil)

	svc, err := uc.GetService(ctx, testCaller, personalNamespace, testRelease)

	require.NoError(t, err)
	assert.Equal(t, "someone-else", svc.Owner)
}

func TestGetService_FieldsMappedFromRecord(t *testing.T) {
	uc, ctx, m := setupReader(t)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, testUsername, true), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
		Return([]ports.PodInfo{{Name: "p", Ready: true}}, nil)

	svc, err := uc.GetService(ctx, testCaller, testNamespace, testRelease)

	require.NoError(t, err)
	assert.Equal(t, testRelease, svc.ReleaseID)
	assert.Equal(t, testNamespace, svc.Namespace)
	assert.Equal(t, "My Service", svc.FriendlyName)
	assert.Equal(t, testUsername, svc.Owner)
	assert.Equal(t, "my-catalog", svc.CatalogID)
	assert.True(t, svc.Share)
}

func TestGetService_HelmStateError(t *testing.T) {
	uc, ctx, m := setupReader(t)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, testUsername, false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{}, errors.New("helm down"))

	_, err := uc.GetService(ctx, testCaller, testNamespace, testRelease)

	assert.ErrorContains(t, err, "helm down")
}

func TestGetService_PodQueryError(t *testing.T) {
	uc, ctx, m := setupReader(t)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, testUsername, false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
		Return(nil, errors.New("k8s down"))

	_, err := uc.GetService(ctx, testCaller, testNamespace, testRelease)

	assert.ErrorContains(t, err, "k8s down")
}

// --- namespace authorization ------------------------------------------------

func TestGetService_ForeignNamespaceForbidden(t *testing.T) {
	uc, ctx, m := setupReader(t)

	_, err := uc.GetService(ctx, testCaller, "user-bob", testRelease)

	assert.ErrorIs(t, err, domain.ErrForbidden)
	m.records.AssertNotCalled(t, "GetServiceRecord", mock.Anything, mock.Anything, mock.Anything)
}

// --- ListServices -----------------------------------------------------------

// listFixture stubs the namespace listings ListServices reads.
type listFixture struct {
	records   []ports.ServiceRecord
	releases  map[string]ports.ReleaseState
	workloads ports.WorkloadReadiness
}

func (f listFixture) stub(m queryMocks, ns string) {
	m.records.On("ListServiceRecords", mock.Anything, ns).Return(f.records, nil)
	if f.releases != nil {
		m.helm.On("ListReleaseStates", mock.Anything, ns).Return(f.releases, nil)
	}
	if f.workloads != nil {
		m.pods.On("GetWorkloadReadiness", mock.Anything, ns).Return(f.workloads, nil)
	}
}

var deployedWithWeb = ports.ReleaseState{
	Exists:    true,
	Status:    ports.ReleaseStatusDeployed,
	Resources: []ports.ManifestResource{{Kind: "Deployment", Name: "web"}},
}

func readiness(ready bool) ports.WorkloadReadiness {
	return ports.WorkloadReadinessFunc(func([]ports.ManifestResource) bool { return ready })
}

func TestListServices_Empty(t *testing.T) {
	uc, ctx, m := setupReader(t)
	listFixture{records: []ports.ServiceRecord{}}.stub(m, testNamespace)

	svcs, err := uc.ListServices(ctx, testCaller, testNamespace)

	require.NoError(t, err)
	assert.Empty(t, svcs)
	m.helm.AssertNotCalled(t, "ListReleaseStates", mock.Anything, mock.Anything)
}

func TestListServices_RecordsListError(t *testing.T) {
	uc, ctx, m := setupReader(t)
	m.records.On("ListServiceRecords", mock.Anything, testNamespace).Return(nil, errors.New("api error"))

	_, err := uc.ListServices(ctx, testCaller, testNamespace)

	assert.ErrorContains(t, err, "api error")
}

func TestListServices_Visibility(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		owner     string
		share     bool
		visible   bool
	}{
		{"owned service", testNamespace, testUsername, false, true},
		{"service shared by another owner", testNamespace, "bob", true, true},
		{"unshared service of another owner", testNamespace, "bob", false, false},
		{"personal namespace shows any owner", personalNamespace, "legacy-owner", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, ctx, m := setupReader(t)
			listFixture{
				records:   []ports.ServiceRecord{record("svc", tt.owner, tt.share)},
				releases:  map[string]ports.ReleaseState{"svc": deployedWithWeb},
				workloads: readiness(true),
			}.stub(m, tt.namespace)

			svcs, err := uc.ListServices(ctx, testCaller, tt.namespace)

			require.NoError(t, err)
			if tt.visible {
				require.Len(t, svcs, 1)
				assert.Equal(t, "svc", svcs[0].ReleaseID)
			} else {
				assert.Empty(t, svcs)
				// Nothing is read for services the caller can't see.
				m.helm.AssertNotCalled(t, "ListReleaseStates", mock.Anything, mock.Anything)
			}
		})
	}
}

func TestListServices_DeployedReleaseFollowsWorkloads(t *testing.T) {
	for _, ready := range []bool{true, false} {
		uc, ctx, m := setupReader(t)
		var gotResources []ports.ManifestResource
		listFixture{
			records:  []ports.ServiceRecord{record(testRelease, testUsername, false)},
			releases: map[string]ports.ReleaseState{testRelease: deployedWithWeb},
			workloads: ports.WorkloadReadinessFunc(func(r []ports.ManifestResource) bool {
				gotResources = r
				return ready
			}),
		}.stub(m, testNamespace)

		svcs, err := uc.ListServices(ctx, testCaller, testNamespace)

		require.NoError(t, err)
		require.Len(t, svcs, 1)
		want := domain.ServiceStatusDeploying
		if ready {
			want = domain.ServiceStatusRunning
		}
		assert.Equal(t, want, svcs[0].Status)
		assert.Equal(t, deployedWithWeb.Resources, gotResources)
	}
}

func TestListServices_RecordWithoutReleaseIsGhost(t *testing.T) {
	uc, ctx, m := setupReader(t)
	listFixture{
		records:  []ports.ServiceRecord{record(testRelease, testUsername, false)},
		releases: map[string]ports.ReleaseState{},
	}.stub(m, testNamespace)

	svcs, err := uc.ListServices(ctx, testCaller, testNamespace)

	require.NoError(t, err)
	require.Len(t, svcs, 1)
	assert.Equal(t, domain.ServiceStatusGhost, svcs[0].Status)
}

// The number of calls doesn't grow with the number of services: one list of
// releases and one workload snapshot, fetched only when a service needs it.
func TestListServices_ConstantNumberOfCalls(t *testing.T) {
	uc, ctx, m := setupReader(t)
	listFixture{
		records: []ports.ServiceRecord{
			record("a", testUsername, false),
			record("b", testUsername, false),
			record("c", testUsername, false),
			record("pending", testUsername, false),
		},
		releases: map[string]ports.ReleaseState{
			"a": deployedWithWeb, "b": deployedWithWeb, "c": deployedWithWeb,
			"pending": {Exists: true, Status: ports.ReleaseStatusPending},
		},
		workloads: readiness(true),
	}.stub(m, testNamespace)

	svcs, err := uc.ListServices(ctx, testCaller, testNamespace)

	require.NoError(t, err)
	require.Len(t, svcs, 4)
	m.helm.AssertNumberOfCalls(t, "ListReleaseStates", 1)
	m.pods.AssertNumberOfCalls(t, "GetWorkloadReadiness", 1)
	m.helm.AssertNotCalled(t, "GetReleaseState", mock.Anything, mock.Anything, mock.Anything)
}

func TestListServices_NoWorkloadCallWhenReleasesDecide(t *testing.T) {
	uc, ctx, m := setupReader(t)
	listFixture{
		records:  []ports.ServiceRecord{record(testRelease, testUsername, false)},
		releases: map[string]ports.ReleaseState{testRelease: {Exists: true, Status: ports.ReleaseStatusPending}},
	}.stub(m, testNamespace)

	svcs, err := uc.ListServices(ctx, testCaller, testNamespace)

	require.NoError(t, err)
	assert.Equal(t, domain.ServiceStatusDeploying, svcs[0].Status)
	m.pods.AssertNotCalled(t, "GetWorkloadReadiness", mock.Anything, mock.Anything)
}

func TestListServices_ReleasesListError(t *testing.T) {
	uc, ctx, m := setupReader(t)
	listFixture{records: []ports.ServiceRecord{record(testRelease, testUsername, false)}}.stub(m, testNamespace)
	m.helm.On("ListReleaseStates", mock.Anything, testNamespace).Return(nil, errors.New("helm down"))

	_, err := uc.ListServices(ctx, testCaller, testNamespace)

	assert.ErrorContains(t, err, "helm down")
}

func TestListServices_WorkloadReadinessError(t *testing.T) {
	uc, ctx, m := setupReader(t)
	listFixture{
		records:  []ports.ServiceRecord{record(testRelease, testUsername, false)},
		releases: map[string]ports.ReleaseState{testRelease: deployedWithWeb},
	}.stub(m, testNamespace)
	m.pods.On("GetWorkloadReadiness", mock.Anything, testNamespace).Return(nil, errors.New("k8s down"))

	_, err := uc.ListServices(ctx, testCaller, testNamespace)

	assert.ErrorContains(t, err, "k8s down")
}

func TestListServices_ForeignNamespaceForbidden(t *testing.T) {
	uc, ctx, m := setupReader(t)

	_, err := uc.ListServices(ctx, testCaller, "projet-other-team")

	assert.ErrorIs(t, err, domain.ErrForbidden)
	m.records.AssertNotCalled(t, "ListServiceRecords", mock.Anything, mock.Anything)
}
