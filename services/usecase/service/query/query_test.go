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

// setupReader creates a Reader and the mocks of its gateways. No release is
// blocked by the quota.
func setupReader(t *testing.T) (*Reader, context.Context, queryMocks) {
	t.Helper()
	return setupReaderWithQuotaFailures(t, map[string]string{})
}

// setupReaderWithQuotaFailures creates a Reader whose workload gateway
// reports quotaFailures.
func setupReaderWithQuotaFailures(t *testing.T, quotaFailures map[string]string) (*Reader, context.Context, queryMocks) {
	t.Helper()
	m := queryMocks{
		helm:    new(mocks.MockReleaseGateway),
		records: new(mocks.MockServiceRecordGateway),
		pods:    new(mocks.MockWorkloadStateGateway),
	}
	m.pods.On("ListQuotaFailures", mock.Anything, mock.Anything).Return(quotaFailures, nil).Maybe()
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

// readerForState drives GetService for a release without pods.
func readerForState(t *testing.T, state ports.ReleaseState) (domain.Service, error) {
	t.Helper()
	uc, ctx, m := setupReader(t)

	m.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
		Return(record(testRelease, testUsername, false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(state, nil)

	if needsWorkloads(state) {
		m.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
			Return([]ports.PodInfo{}, nil)
	}

	return uc.GetService(ctx, testCaller, testNamespace, testRelease)
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
	records  []ports.ServiceRecord
	releases map[string]ports.ReleaseState
	pods     map[string][]ports.PodInfo
}

func (f listFixture) stub(m queryMocks, ns string) {
	m.records.On("ListServiceRecords", mock.Anything, ns).Return(f.records, nil)
	if f.releases != nil {
		m.helm.On("ListReleaseStates", mock.Anything, ns).Return(f.releases, nil)
	}
	if f.pods != nil {
		m.pods.On("ListPodsByRelease", mock.Anything, ns).Return(f.pods, nil)
	}
}

var deployed = ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}

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
				records:  []ports.ServiceRecord{record("svc", tt.owner, tt.share)},
				releases: map[string]ports.ReleaseState{"svc": deployed},
				pods:     map[string][]ports.PodInfo{"svc": {{Name: "p", Ready: true}}},
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

// GetService and ListServices derive the same status from the same cluster
// state; only GetService carries the error detail.
func TestGetAndListServicesAgree(t *testing.T) {
	tests := []struct {
		name         string
		release      ports.ReleaseState
		pods         []ports.PodInfo
		quotaFailure string
	}{
		{"ghost", ports.ReleaseState{}, nil, ""},
		{"uninstalled, pods shutting down", ports.ReleaseState{}, []ports.PodInfo{{Name: "p", Terminating: true}}, ""},
		{"blocked by the quota", deployed, nil, "exceeded quota"},
		{"failed release", ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusFailed, Message: "boom"}, nil, ""},
		{"pending", ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusPending}, nil, ""},
		{"uninstalling", ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusUninstalling}, nil, ""},
		{"suspending", ports.ReleaseState{Exists: true, Suspended: true, Status: ports.ReleaseStatusDeployed}, []ports.PodInfo{{Name: "p", Terminating: true}}, ""},
		{"suspended", ports.ReleaseState{Exists: true, Suspended: true, Status: ports.ReleaseStatusDeployed}, nil, ""},
		{"deployed, no pod", deployed, nil, ""},
		{"deployed, ready", deployed, []ports.PodInfo{{Name: "p", Ready: true}}, ""},
		{"deployed, starting", deployed, []ports.PodInfo{{Name: "p"}}, ""},
		{"deployed, crash loop", deployed, []ports.PodInfo{{Name: "p", ErrorReason: domain.ServiceErrorReasonCrashLoop}}, ""},
		{"deployed, rollout", deployed, []ports.PodInfo{{Name: "new", Ready: true}, {Name: "old", Terminating: true}}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failures := map[string]string{}
			if tt.quotaFailure != "" {
				failures[testRelease] = tt.quotaFailure
			}
			getUC, ctx, getMocks := setupReaderWithQuotaFailures(t, failures)
			getMocks.records.On("GetServiceRecord", mock.Anything, testNamespace, testRelease).
				Return(record(testRelease, testUsername, false), nil)
			getMocks.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
				Return(tt.release, nil)
			getMocks.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
				Return(tt.pods, nil).Maybe()

			got, err := getUC.GetService(ctx, testCaller, testNamespace, testRelease)
			require.NoError(t, err)

			releases := map[string]ports.ReleaseState{}
			if tt.release.Exists {
				releases[testRelease] = tt.release
			}
			listUC, ctx, listMocks := setupReaderWithQuotaFailures(t, failures)
			listFixture{
				records:  []ports.ServiceRecord{record(testRelease, testUsername, false)},
				releases: releases,
				pods:     map[string][]ports.PodInfo{testRelease: tt.pods},
			}.stub(listMocks, testNamespace)

			listed, err := listUC.ListServices(ctx, testCaller, testNamespace)
			require.NoError(t, err)
			require.Len(t, listed, 1)

			assert.Equal(t, got.Status, listed[0].Status)
			assert.Equal(t, got.Status == domain.ServiceStatusError, got.Error != nil)
			assert.Nil(t, listed[0].Error)
		})
	}
}

func TestListServices_RecordWithoutReleaseIsGhost(t *testing.T) {
	uc, ctx, m := setupReader(t)
	listFixture{
		records:  []ports.ServiceRecord{record(testRelease, testUsername, false)},
		releases: map[string]ports.ReleaseState{},
		pods:     map[string][]ports.PodInfo{},
	}.stub(m, testNamespace)

	svcs, err := uc.ListServices(ctx, testCaller, testNamespace)

	require.NoError(t, err)
	require.Len(t, svcs, 1)
	assert.Equal(t, domain.ServiceStatusGhost, svcs[0].Status)
}

// The number of calls doesn't grow with the number of services: one list of
// releases and one of the pods, fetched only when a service needs it.
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
			"a": deployed, "b": deployed, "c": deployed,
			"pending": {Exists: true, Status: ports.ReleaseStatusPending},
		},
		pods: map[string][]ports.PodInfo{},
	}.stub(m, testNamespace)

	svcs, err := uc.ListServices(ctx, testCaller, testNamespace)

	require.NoError(t, err)
	require.Len(t, svcs, 4)
	m.helm.AssertNumberOfCalls(t, "ListReleaseStates", 1)
	m.pods.AssertNumberOfCalls(t, "ListPodsByRelease", 1)
	m.pods.AssertNotCalled(t, "GetPodsForRelease", mock.Anything, mock.Anything, mock.Anything)
	m.helm.AssertNotCalled(t, "GetReleaseState", mock.Anything, mock.Anything, mock.Anything)
}

func TestListServices_NoPodCallWhenReleasesDecide(t *testing.T) {
	uc, ctx, m := setupReader(t)
	listFixture{
		records:  []ports.ServiceRecord{record(testRelease, testUsername, false)},
		releases: map[string]ports.ReleaseState{testRelease: {Exists: true, Status: ports.ReleaseStatusPending}},
	}.stub(m, testNamespace)

	svcs, err := uc.ListServices(ctx, testCaller, testNamespace)

	require.NoError(t, err)
	assert.Equal(t, domain.ServiceStatusDeploying, svcs[0].Status)
	m.pods.AssertNotCalled(t, "ListPodsByRelease", mock.Anything, mock.Anything)
}

func TestListServices_ReleasesListError(t *testing.T) {
	uc, ctx, m := setupReader(t)
	listFixture{records: []ports.ServiceRecord{record(testRelease, testUsername, false)}}.stub(m, testNamespace)
	m.helm.On("ListReleaseStates", mock.Anything, testNamespace).Return(nil, errors.New("helm down"))

	_, err := uc.ListServices(ctx, testCaller, testNamespace)

	assert.ErrorContains(t, err, "helm down")
}

func TestListServices_PodsListError(t *testing.T) {
	uc, ctx, m := setupReader(t)
	listFixture{
		records:  []ports.ServiceRecord{record(testRelease, testUsername, false)},
		releases: map[string]ports.ReleaseState{testRelease: deployed},
	}.stub(m, testNamespace)
	m.pods.On("ListPodsByRelease", mock.Anything, testNamespace).Return(nil, errors.New("k8s down"))

	_, err := uc.ListServices(ctx, testCaller, testNamespace)

	assert.ErrorContains(t, err, "k8s down")
}

func TestListServices_ForeignNamespaceForbidden(t *testing.T) {
	uc, ctx, m := setupReader(t)

	_, err := uc.ListServices(ctx, testCaller, "projet-other-team")

	assert.ErrorIs(t, err, domain.ErrForbidden)
	m.records.AssertNotCalled(t, "ListServiceRecords", mock.Anything, mock.Anything)
}
