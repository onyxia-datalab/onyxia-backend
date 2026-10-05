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
	secrets *mocks.MockOnyxiaSecretGateway
	pods    *mocks.MockWorkloadStateGateway
}

// setupReader creates a Reader with a context that carries the given username.
func setupReader(t *testing.T, username string) (*Reader, context.Context, queryMocks) {
	t.Helper()
	m := queryMocks{
		helm:    new(mocks.MockReleaseGateway),
		secrets: new(mocks.MockOnyxiaSecretGateway),
		pods:    new(mocks.MockWorkloadStateGateway),
	}
	ctx, reader, _ := usercontext.NewTestUserContext(&usercontext.User{Username: username})
	uc := NewReader(m.secrets, m.helm, m.pods, reader, namespace.NewAuthorizer("user-", "projet-"))
	return uc, ctx, m
}

const (
	// A group namespace: the owner/share filter only applies there.
	testNamespace     = "projet-data-team"
	personalNamespace = "user-alice"
	testRelease       = "jupyter-abc"
	testUsername      = "alice"
)

func secretData(owner string, share bool) map[string][]byte {
	shareStr := "false"
	if share {
		shareStr = "true"
	}
	return map[string][]byte{
		"friendlyName": []byte("My Service"),
		"owner":        []byte(owner),
		"catalog":      []byte("my-catalog"),
		"share":        []byte(shareStr),
	}
}

// readerForState drives GetService through the Helm-state path.
// Covers Ghost (Exists=false) and Suspended (both decided by statusFromRelease).
func readerForState(t *testing.T, state ports.ReleaseState) (domain.Service, error) {
	t.Helper()
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData(testUsername, false), nil)
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

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, testNamespace).
		Return([]string{testRelease}, nil)
	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData(testUsername, false), nil)
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

	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData(testUsername, false), nil)
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

func TestListServices_SecretReadError(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, testNamespace).
		Return([]string{testRelease}, nil)
	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(nil, errors.New("k8s down"))

	_, err := uc.ListServices(ctx, testNamespace)

	assert.ErrorContains(t, err, "k8s down")
}

func TestGetService_SecretNotFound(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(nil, domain.ErrNotFound)

	_, err := uc.GetService(ctx, testNamespace, testRelease)

	assert.ErrorIs(t, err, domain.ErrNotFound)
	m.helm.AssertNotCalled(t, "GetReleaseState")
}

func TestGetService_SecretReadError(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(nil, errors.New("k8s unavailable"))

	_, err := uc.GetService(ctx, testNamespace, testRelease)

	assert.ErrorContains(t, err, "k8s unavailable")
}

func TestGetService_DeniesUnownedUnshared(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData("someone-else", false), nil)

	_, err := uc.GetService(ctx, testNamespace, testRelease)

	assert.ErrorIs(t, err, domain.ErrNotFound)
	m.helm.AssertNotCalled(t, "GetReleaseState")
}

func TestGetService_AllowsSharedFromOtherOwner(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData("someone-else", true), nil)
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

	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData("ALICE", false), nil)
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

	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, personalNamespace, testRelease).
		Return(secretData("someone-else", false), nil)
	m.helm.On("GetReleaseState", mock.Anything, personalNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetPodsForRelease", mock.Anything, personalNamespace, testRelease).
		Return([]ports.PodInfo{}, nil)

	svc, err := uc.GetService(ctx, personalNamespace, testRelease)

	require.NoError(t, err)
	assert.Equal(t, "someone-else", svc.Owner)
}

func TestGetService_FieldsMappedFromSecret(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData(testUsername, true), nil)
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

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, testNamespace).
		Return([]string{}, nil)

	svcs, err := uc.ListServices(ctx, testNamespace)

	require.NoError(t, err)
	assert.Empty(t, svcs)
}

func TestListServices_ListError(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, testNamespace).
		Return(nil, errors.New("api error"))

	_, err := uc.ListServices(ctx, testNamespace)

	assert.ErrorContains(t, err, "api error")
}

func TestListServices_FiltersOutOtherOwnerUnshared(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, testNamespace).
		Return([]string{"svc-bob"}, nil)
	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, "svc-bob").
		Return(secretData("bob", false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, "svc-bob").
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetControllerReadiness", mock.Anything, testNamespace, mock.Anything).
		Return(true, nil)
	m.helm.On("GetReleaseResources", mock.Anything, testNamespace, "svc-bob").
		Return([]ports.ManifestResource{}, nil)

	svcs, err := uc.ListServices(ctx, testNamespace)

	require.NoError(t, err)
	assert.Empty(t, svcs)
}

func TestListServices_IncludesOwnedService(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, testNamespace).
		Return([]string{testRelease}, nil)
	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData(testUsername, false), nil)
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

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, testNamespace).
		Return([]string{"svc-bob-shared"}, nil)
	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, "svc-bob-shared").
		Return(secretData("bob", true), nil)
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

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, personalNamespace).
		Return([]string{"svc-legacy"}, nil)
	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, personalNamespace, "svc-legacy").
		Return(secretData("legacy-owner", false), nil)
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

func TestListServices_SkipsSecretDisappearedRace(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, testNamespace).
		Return([]string{"ghost-svc"}, nil)
	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, "ghost-svc").
		Return(nil, domain.ErrNotFound)

	svcs, err := uc.ListServices(ctx, testNamespace)

	require.NoError(t, err)
	assert.Empty(t, svcs)
}

// --- deriveStatusLight (via ListServices) -----------------------------------

func TestListServices_DeployedRelease_Ready(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	resources := []ports.ManifestResource{{Kind: "Deployment", Name: "my-deploy"}}

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, testNamespace).
		Return([]string{testRelease}, nil)
	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData(testUsername, false), nil)
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

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, testNamespace).
		Return([]string{testRelease}, nil)
	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData(testUsername, false), nil)
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

	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData(testUsername, false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{}, errors.New("helm down"))

	_, err := uc.GetService(ctx, testNamespace, testRelease)

	assert.ErrorContains(t, err, "helm down")
}

func TestGetService_PodQueryError(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData(testUsername, false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.pods.On("GetPodsForRelease", mock.Anything, testNamespace, testRelease).
		Return(nil, errors.New("k8s down"))

	_, err := uc.GetService(ctx, testNamespace, testRelease)

	assert.ErrorContains(t, err, "k8s down")
}

func TestListServices_HelmStateError(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, testNamespace).
		Return([]string{testRelease}, nil)
	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData(testUsername, false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{}, errors.New("helm down"))

	_, err := uc.ListServices(ctx, testNamespace)

	assert.ErrorContains(t, err, "helm down")
}

func TestListServices_ReleaseResourcesError(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, testNamespace).
		Return([]string{testRelease}, nil)
	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData(testUsername, false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}, nil)
	m.helm.On("GetReleaseResources", mock.Anything, testNamespace, testRelease).
		Return(nil, errors.New("k8s down"))

	_, err := uc.ListServices(ctx, testNamespace)

	assert.ErrorContains(t, err, "k8s down")
}

func TestListServices_ControllerReadinessError(t *testing.T) {
	uc, ctx, m := setupReader(t, testUsername)

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, testNamespace).
		Return([]string{testRelease}, nil)
	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData(testUsername, false), nil)
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

	m.secrets.On("ListOnyxiaSecretNames", mock.Anything, testNamespace).
		Return([]string{testRelease}, nil)
	m.secrets.On("ReadOnyxiaSecretData", mock.Anything, testNamespace, testRelease).
		Return(secretData(testUsername, false), nil)
	m.helm.On("GetReleaseState", mock.Anything, testNamespace, testRelease).
		Return(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusPending}, nil)

	svcs, err := uc.ListServices(ctx, testNamespace)

	require.NoError(t, err)
	require.Len(t, svcs, 1)
	assert.Equal(t, domain.ServiceStatusDeploying, svcs[0].Status)
	m.pods.AssertNotCalled(t, "GetControllerReadiness")
	m.helm.AssertNotCalled(t, "GetReleaseResources")
}
