package lifecycle

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

type lifecycleMocks struct {
	helm    *mocks.MockReleaseGateway
	records *mocks.MockServiceRecordGateway
	catalog *mocks.MockCatalogService
}

func setupLifecycle(t *testing.T) (*Lifecycle, context.Context, lifecycleMocks) {
	t.Helper()
	m := lifecycleMocks{
		helm:    new(mocks.MockReleaseGateway),
		records: new(mocks.MockServiceRecordGateway),
		catalog: new(mocks.MockCatalogService),
	}
	uc := NewLifecycle(m.records, m.helm, m.catalog, namespace.NewAuthorizer("user-", "projet-"))
	return uc, context.Background(), m
}

// alice may act in her personal namespace and in groupNamespace.
var alice = usercontext.User{Username: "alice", Groups: []string{"data-team"}}

func baseRequest() domain.StartRequest {
	return domain.StartRequest{
		User:         alice,
		CatalogID:    "my-catalog",
		PackageName:  "jupyter-python",
		Version:      "1.0.0",
		ReleaseID:    "release-abc",
		Namespace:    "user-alice",
		FriendlyName: "My Jupyter",
		Name:         "jupyter-alice",
		Share:        false,
		Values:       map[string]interface{}{"key": "val"},
	}
}

func resolvedPkg(req domain.StartRequest) domain.Package {
	return domain.Package{
		Name:      req.PackageName,
		CatalogID: req.CatalogID,
		RepoURL:   "https://charts.example.com",
	}
}

// notInstalled stubs GetReleaseState to report the release doesn't exist yet,
// the common case exercised by most Start tests.
func notInstalled(m lifecycleMocks, req domain.StartRequest) {
	m.helm.On("GetReleaseState", mock.Anything, req.Namespace, req.ReleaseID).
		Return(ports.ReleaseState{Exists: false}, nil)
}

func TestStart_Success(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()
	pkg := resolvedPkg(req)

	m.catalog.On("GetPackage", ctx, &req.User, req.CatalogID, req.PackageName).Return(pkg, nil)
	notInstalled(m, req)
	m.records.On("CreateServiceRecord", ctx, req.Namespace, mock.Anything).Return(nil)
	m.helm.On("StartInstall", ctx, req.Namespace, req.ReleaseID, mock.Anything, req.Version, req.Values).
		Return(nil)

	err := uc.Start(ctx, req)

	require.NoError(t, err)
	m.catalog.AssertExpectations(t)
	m.records.AssertExpectations(t)
	m.helm.AssertExpectations(t)
}

func TestStart_ServiceRecordIsCorrect(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()
	req.Share = true
	pkg := resolvedPkg(req)

	m.catalog.On("GetPackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(pkg, nil)
	m.catalog.On("CheckSharingAllowed", mock.Anything, mock.Anything, req.CatalogID).Return(nil)
	notInstalled(m, req)
	m.records.On("CreateServiceRecord", ctx, req.Namespace, ports.ServiceRecord{
		ReleaseID:    req.ReleaseID,
		CatalogID:    req.CatalogID,
		FriendlyName: req.FriendlyName,
		Owner:        req.User.Username,
		Share:        true,
	}).Return(nil)
	m.helm.On("StartInstall", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	err := uc.Start(ctx, req)

	require.NoError(t, err)
	m.records.AssertExpectations(t)
}

func TestStart_GetPackageError(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()

	m.catalog.On("GetPackage", ctx, &req.User, req.CatalogID, req.PackageName).
		Return(domain.Package{}, errors.New("index unavailable"))

	err := uc.Start(ctx, req)

	assert.ErrorContains(t, err, "index unavailable")
	m.records.AssertNotCalled(t, "CreateServiceRecord")
	m.helm.AssertNotCalled(t, "StartInstall")
}

func TestStart_PackageNotFound(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()

	m.catalog.On("GetPackage", ctx, &req.User, req.CatalogID, req.PackageName).
		Return(domain.Package{}, domain.ErrNotFound)

	err := uc.Start(ctx, req)

	assert.ErrorIs(t, err, domain.ErrNotFound)
	m.records.AssertNotCalled(t, "CreateServiceRecord")
	m.helm.AssertNotCalled(t, "StartInstall")
}

// A restricted catalog the caller can't access is reported by GetPackage as
// ErrNotFound, so the install path is naturally covered by the same case as
// TestStart_PackageNotFound: catalog.GetPackage is where the restriction is
// enforced, not Start.

func TestStart_SharingNotAllowed(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()
	req.Share = true
	pkg := resolvedPkg(req)

	m.catalog.On("GetPackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(pkg, nil)
	m.catalog.On("CheckSharingAllowed", mock.Anything, mock.Anything, req.CatalogID).Return(domain.ErrForbidden)

	err := uc.Start(ctx, req)

	assert.ErrorIs(t, err, domain.ErrForbidden)
	m.helm.AssertNotCalled(t, "GetReleaseState")
	m.records.AssertNotCalled(t, "CreateServiceRecord")
	m.helm.AssertNotCalled(t, "StartInstall")
}

func TestStart_ShareFalseSkipsSharingCheck(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()
	req.Share = false
	pkg := resolvedPkg(req)

	m.catalog.On("GetPackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(pkg, nil)
	notInstalled(m, req)
	m.records.On("CreateServiceRecord", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	m.helm.On("StartInstall", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	err := uc.Start(ctx, req)

	require.NoError(t, err)
	m.catalog.AssertNotCalled(t, "CheckSharingAllowed", mock.Anything, mock.Anything, mock.Anything)
}

func TestStart_AlreadyExists(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()
	pkg := resolvedPkg(req)

	m.catalog.On("GetPackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(pkg, nil)
	m.helm.On("GetReleaseState", mock.Anything, req.Namespace, req.ReleaseID).
		Return(ports.ReleaseState{Exists: true}, nil)

	err := uc.Start(ctx, req)

	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
	m.records.AssertNotCalled(t, "CreateServiceRecord")
	m.helm.AssertNotCalled(t, "StartInstall")
}

func TestStart_GetReleaseStateError(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()
	pkg := resolvedPkg(req)

	m.catalog.On("GetPackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(pkg, nil)
	m.helm.On("GetReleaseState", mock.Anything, req.Namespace, req.ReleaseID).
		Return(ports.ReleaseState{}, errors.New("helm unavailable"))

	err := uc.Start(ctx, req)

	assert.ErrorContains(t, err, "helm unavailable")
	m.records.AssertNotCalled(t, "CreateServiceRecord")
	m.helm.AssertNotCalled(t, "StartInstall")
}

func TestStart_RecordError(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()
	pkg := resolvedPkg(req)

	m.catalog.On("GetPackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(pkg, nil)
	notInstalled(m, req)
	m.records.On("CreateServiceRecord", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("k8s unavailable"))

	err := uc.Start(ctx, req)

	assert.ErrorContains(t, err, "k8s unavailable")
	m.helm.AssertNotCalled(t, "StartInstall")
}

func TestStart_RecordAlreadyExistsDoesNotOverwriteOrInstall(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()
	pkg := resolvedPkg(req)

	m.catalog.On("GetPackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(pkg, nil)
	notInstalled(m, req)
	m.records.On("CreateServiceRecord", mock.Anything, req.Namespace, mock.Anything).
		Return(domain.ErrAlreadyExists)

	err := uc.Start(ctx, req)

	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
	m.helm.AssertNotCalled(t, "StartInstall")
}

func TestStart_HelmError(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()
	pkg := resolvedPkg(req)

	m.catalog.On("GetPackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(pkg, nil)
	notInstalled(m, req)
	m.records.On("CreateServiceRecord", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	m.helm.On("StartInstall", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("invalid release name"))

	err := uc.Start(ctx, req)

	assert.ErrorContains(t, err, "invalid release name")
}

// --- Suspend / Resume / Delete: ownership ---------------------------------

const (
	groupNamespace = "projet-data-team"
	release        = "jupyter-alice"
)

// ownedBy stubs the service record of release in ns with the given owner/share.
func ownedBy(m lifecycleMocks, ns, owner string, share bool) {
	m.records.On("GetServiceRecord", mock.Anything, ns, release).
		Return(ports.ServiceRecord{ReleaseID: release, Owner: owner, Share: share}, nil)
}

func TestAuthorize_Rules(t *testing.T) {
	tests := []struct {
		name    string
		ns      string
		owner   string
		share   bool
		allowed bool
	}{
		{"owner in group namespace", groupNamespace, "alice", false, true},
		{"owner case-insensitive", groupNamespace, "ALICE", false, true},
		{"shared by someone else", groupNamespace, "bob", true, true},
		{"unshared by someone else", groupNamespace, "bob", false, false},
		{"personal namespace, any owner", "user-alice", "bob", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, ctx, m := setupLifecycle(t)
			ownedBy(m, tt.ns, tt.owner, tt.share)

			_, err := uc.authorize(ctx, alice, tt.ns, release)

			if tt.allowed {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, domain.ErrNotFound)
			}
		})
	}
}

func TestAuthorize_RecordNotFound(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	m.records.On("GetServiceRecord", mock.Anything, groupNamespace, release).
		Return(ports.ServiceRecord{}, domain.ErrNotFound)

	_, err := uc.authorize(ctx, alice, groupNamespace, release)

	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestAuthorize_RecordReadError(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	m.records.On("GetServiceRecord", mock.Anything, groupNamespace, release).
		Return(ports.ServiceRecord{}, errors.New("k8s unavailable"))

	_, err := uc.authorize(ctx, alice, groupNamespace, release)

	assert.ErrorContains(t, err, "k8s unavailable")
	assert.NotErrorIs(t, err, domain.ErrNotFound)
}

// --- Suspend ----------------------------------------------------------------

func TestSuspend_Success(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedBy(m, groupNamespace, "alice", false)
	m.helm.On("SuspendRelease", ctx, groupNamespace, release).Return(nil)

	err := uc.Suspend(ctx, domain.SuspendRequest{User: alice, Namespace: groupNamespace, ReleaseName: release})

	require.NoError(t, err)
	m.helm.AssertExpectations(t)
}

func TestSuspend_DeniedForUnsharedForeignService(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedBy(m, groupNamespace, "bob", false)

	err := uc.Suspend(ctx, domain.SuspendRequest{User: alice, Namespace: groupNamespace, ReleaseName: release})

	assert.ErrorIs(t, err, domain.ErrNotFound)
	m.helm.AssertNotCalled(t, "SuspendRelease")
}

func TestSuspend_HelmError(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedBy(m, groupNamespace, "alice", false)
	m.helm.On("SuspendRelease", ctx, groupNamespace, release).
		Return(errors.New("helm unavailable"))

	err := uc.Suspend(ctx, domain.SuspendRequest{User: alice, Namespace: groupNamespace, ReleaseName: release})

	assert.ErrorContains(t, err, "helm unavailable")
}

// --- Resume -----------------------------------------------------------------

func TestResume_Success(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedBy(m, groupNamespace, "bob", true)
	m.helm.On("ResumeRelease", ctx, groupNamespace, release).Return(nil)

	err := uc.Resume(ctx, domain.ResumeRequest{User: alice, Namespace: groupNamespace, ReleaseName: release})

	require.NoError(t, err)
	m.helm.AssertExpectations(t)
}

func TestResume_DeniedForUnsharedForeignService(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedBy(m, groupNamespace, "bob", false)

	err := uc.Resume(ctx, domain.ResumeRequest{User: alice, Namespace: groupNamespace, ReleaseName: release})

	assert.ErrorIs(t, err, domain.ErrNotFound)
	m.helm.AssertNotCalled(t, "ResumeRelease")
}

func TestResume_HelmError(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedBy(m, groupNamespace, "alice", false)
	m.helm.On("ResumeRelease", ctx, groupNamespace, release).
		Return(errors.New("helm unavailable"))

	err := uc.Resume(ctx, domain.ResumeRequest{User: alice, Namespace: groupNamespace, ReleaseName: release})

	assert.ErrorContains(t, err, "helm unavailable")
}

// --- Delete -----------------------------------------------------------------

func TestDelete_Success(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedBy(m, groupNamespace, "alice", false)
	m.helm.On("UninstallRelease", ctx, groupNamespace, release).Return(nil)
	m.records.On("DeleteServiceRecord", ctx, groupNamespace, release).Return(nil)

	err := uc.Delete(ctx, domain.DeleteRequest{User: alice, Namespace: groupNamespace, ReleaseName: release})

	require.NoError(t, err)
	m.helm.AssertExpectations(t)
	m.records.AssertExpectations(t)
}

func TestDelete_DeniedForUnsharedForeignService(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedBy(m, groupNamespace, "bob", false)

	err := uc.Delete(ctx, domain.DeleteRequest{User: alice, Namespace: groupNamespace, ReleaseName: release})

	assert.ErrorIs(t, err, domain.ErrNotFound)
	m.helm.AssertNotCalled(t, "UninstallRelease")
	m.records.AssertNotCalled(t, "DeleteServiceRecord")
}

func TestDelete_HelmError(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedBy(m, groupNamespace, "alice", false)
	m.helm.On("UninstallRelease", ctx, groupNamespace, release).
		Return(errors.New("helm unavailable"))

	err := uc.Delete(ctx, domain.DeleteRequest{User: alice, Namespace: groupNamespace, ReleaseName: release})

	assert.ErrorContains(t, err, "helm unavailable")
	m.records.AssertNotCalled(t, "DeleteServiceRecord")
}

func TestDelete_RecordError(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedBy(m, groupNamespace, "alice", false)
	m.helm.On("UninstallRelease", ctx, groupNamespace, release).Return(nil)
	m.records.On("DeleteServiceRecord", ctx, groupNamespace, release).
		Return(errors.New("k8s unavailable"))

	err := uc.Delete(ctx, domain.DeleteRequest{User: alice, Namespace: groupNamespace, ReleaseName: release})

	assert.ErrorContains(t, err, "k8s unavailable")
}

// --- SetShared --------------------------------------------------------------

func sharedReq(shared bool) domain.SetSharedRequest {
	return domain.SetSharedRequest{
		User:        alice,
		Namespace:   groupNamespace,
		ReleaseName: release,
		Shared:      shared,
	}
}

// ownedWithCatalog stubs the service record including its catalog.
func ownedWithCatalog(m lifecycleMocks, owner string, share bool) {
	m.records.On("GetServiceRecord", mock.Anything, groupNamespace, release).
		Return(ports.ServiceRecord{ReleaseID: release, Owner: owner, Share: share, CatalogID: "my-catalog"}, nil)
}

func TestSetShared_OwnerShares(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedWithCatalog(m, "alice", false)
	m.catalog.On("CheckSharingAllowed", ctx, mock.Anything, "my-catalog").Return(nil)
	m.records.On("SetServiceShared", ctx, groupNamespace, release, true).Return(nil)

	err := uc.SetShared(ctx, sharedReq(true))

	require.NoError(t, err)
	m.catalog.AssertExpectations(t)
	m.records.AssertExpectations(t)
}

func TestSetShared_OwnerUnsharesWithoutCatalogCheck(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedWithCatalog(m, "ALICE", true)
	m.records.On("SetServiceShared", ctx, groupNamespace, release, false).Return(nil)

	err := uc.SetShared(ctx, sharedReq(false))

	require.NoError(t, err)
	m.catalog.AssertNotCalled(t, "CheckSharingAllowed")
}

func TestSetShared_NonOwnerOfSharedServiceForbidden(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedWithCatalog(m, "bob", true)

	err := uc.SetShared(ctx, sharedReq(false))

	assert.ErrorIs(t, err, domain.ErrForbidden)
	m.records.AssertNotCalled(t, "SetServiceShared")
}

func TestSetShared_InvisibleServiceNotFound(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedWithCatalog(m, "bob", false)

	err := uc.SetShared(ctx, sharedReq(true))

	assert.ErrorIs(t, err, domain.ErrNotFound)
	m.records.AssertNotCalled(t, "SetServiceShared")
}

func TestSetShared_CatalogDisallowsSharing(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedWithCatalog(m, "alice", false)
	m.catalog.On("CheckSharingAllowed", ctx, mock.Anything, "my-catalog").
		Return(domain.ErrForbidden)

	err := uc.SetShared(ctx, sharedReq(true))

	assert.ErrorIs(t, err, domain.ErrForbidden)
	m.records.AssertNotCalled(t, "SetServiceShared")
}

func TestSetShared_UpdateError(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	ownedWithCatalog(m, "alice", true)
	m.records.On("SetServiceShared", ctx, groupNamespace, release, mock.Anything).
		Return(errors.New("k8s unavailable"))

	err := uc.SetShared(ctx, sharedReq(false))

	assert.ErrorContains(t, err, "k8s unavailable")
}

// --- namespace authorization ------------------------------------------------

func TestLifecycle_ForeignNamespaceForbidden(t *testing.T) {
	const foreign = "projet-other-team"
	tests := map[string]func(*Lifecycle, context.Context) error{
		"start": func(uc *Lifecycle, ctx context.Context) error {
			req := baseRequest()
			req.Namespace = foreign
			err := uc.Start(ctx, req)
			return err
		},
		"suspend": func(uc *Lifecycle, ctx context.Context) error {
			return uc.Suspend(ctx, domain.SuspendRequest{User: alice, Namespace: foreign, ReleaseName: release})
		},
		"resume": func(uc *Lifecycle, ctx context.Context) error {
			return uc.Resume(ctx, domain.ResumeRequest{User: alice, Namespace: foreign, ReleaseName: release})
		},
		"delete": func(uc *Lifecycle, ctx context.Context) error {
			return uc.Delete(ctx, domain.DeleteRequest{User: alice, Namespace: foreign, ReleaseName: release})
		},
		"set shared": func(uc *Lifecycle, ctx context.Context) error {
			return uc.SetShared(ctx, domain.SetSharedRequest{User: alice, Namespace: foreign, ReleaseName: release})
		},
	}

	for name, call := range tests {
		t.Run(name, func(t *testing.T) {
			// No expectation is set on the mocks: any gateway call fails the test.
			uc, ctx, _ := setupLifecycle(t)

			assert.ErrorIs(t, call(uc, ctx), domain.ErrForbidden)
		})
	}
}
