package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ---------- Mocks ----------

type MockHelmReleasesGateway struct{ mock.Mock }

var _ ports.HelmReleasesGateway = (*MockHelmReleasesGateway)(nil)

func (m *MockHelmReleasesGateway) StartInstall(
	ctx context.Context,
	releaseName string,
	pkg *domain.Package,
	version string,
	vals map[string]interface{},
	opts ports.HelmStartOptions,
) error {
	return m.Called(ctx, releaseName, pkg, version, vals, opts).Error(0)
}

type MockOnyxiaSecretGateway struct{ mock.Mock }

var _ ports.OnyxiaSecretGateway = (*MockOnyxiaSecretGateway)(nil)

func (m *MockOnyxiaSecretGateway) EnsureOnyxiaSecret(
	ctx context.Context,
	namespace, name string,
	data map[string][]byte,
) error {
	return m.Called(ctx, namespace, name, data).Error(0)
}

func (m *MockOnyxiaSecretGateway) DeleteOnyxiaSecret(ctx context.Context, namespace, name string) error {
	return m.Called(ctx, namespace, name).Error(0)
}

func (m *MockOnyxiaSecretGateway) ReadOnyxiaSecretData(
	ctx context.Context,
	namespace, name string,
) (map[string][]byte, error) {
	args := m.Called(ctx, namespace, name)
	if v := args.Get(0); v != nil {
		return v.(map[string][]byte), args.Error(1)
	}
	return nil, args.Error(1)
}

// ---------- Setup ----------

type serviceLifecycleMocks struct {
	helm    *MockHelmReleasesGateway
	secrets *MockOnyxiaSecretGateway
	pkgRepo *MockCatalogRepository
}

func setupServiceLifecycle(t *testing.T) (*ServiceLifecycle, context.Context, serviceLifecycleMocks) {
	t.Helper()
	mocks := serviceLifecycleMocks{
		helm:    new(MockHelmReleasesGateway),
		secrets: new(MockOnyxiaSecretGateway),
		pkgRepo: new(MockCatalogRepository),
	}
	uc := NewServiceLifecycle(mocks.secrets, mocks.helm, mocks.pkgRepo)
	return uc, context.Background(), mocks
}

func baseRequest() domain.StartRequest {
	return domain.StartRequest{
		Username:     "alice",
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

// ---------- Tests ----------

// ✅ Happy path: all steps succeed.
func TestStart_Success(t *testing.T) {
	uc, ctx, m := setupServiceLifecycle(t)
	req := baseRequest()
	pkg := resolvedPkg(req)

	m.pkgRepo.On("GetPackage", ctx, req.CatalogID, req.PackageName).
		Return(pkg, nil)
	m.secrets.On("EnsureOnyxiaSecret", ctx, req.Namespace, req.ReleaseID, mock.Anything).
		Return(nil)
	m.helm.On("StartInstall", ctx, req.Name, mock.Anything, req.Version, req.Values, mock.Anything).
		Return(nil)

	_, err := uc.Start(ctx, req)

	require.NoError(t, err)
	m.pkgRepo.AssertExpectations(t)
	m.secrets.AssertExpectations(t)
	m.helm.AssertExpectations(t)
}

// ✅ Secret data contains the expected fields.
func TestStart_SecretDataIsCorrect(t *testing.T) {
	uc, ctx, m := setupServiceLifecycle(t)
	req := baseRequest()
	req.Share = true
	pkg := resolvedPkg(req)

	m.pkgRepo.On("GetPackage", mock.Anything, mock.Anything, mock.Anything).
		Return(pkg, nil)
	m.secrets.On("EnsureOnyxiaSecret", ctx, req.Namespace, req.ReleaseID,
		map[string][]byte{
			"catalog":      []byte(req.CatalogID),
			"friendlyName": []byte(req.FriendlyName),
			"owner":        []byte(req.Username),
			"share":        []byte("true"),
		},
	).Return(nil)
	m.helm.On("StartInstall", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	_, err := uc.Start(ctx, req)

	require.NoError(t, err)
	m.secrets.AssertExpectations(t)
}

// ❌ GetPackage fails → error propagated, no further calls.
func TestStart_GetPackageError(t *testing.T) {
	uc, ctx, m := setupServiceLifecycle(t)
	req := baseRequest()

	m.pkgRepo.On("GetPackage", ctx, req.CatalogID, req.PackageName).
		Return(domain.Package{}, errors.New("index unavailable"))

	_, err := uc.Start(ctx, req)

	assert.ErrorContains(t, err, "index unavailable")
	m.secrets.AssertNotCalled(t, "EnsureOnyxiaSecret")
	m.helm.AssertNotCalled(t, "StartInstall")
}

// ❌ GetPackage returns ErrNotFound → propagated.
func TestStart_PackageNotFound(t *testing.T) {
	uc, ctx, m := setupServiceLifecycle(t)
	req := baseRequest()

	m.pkgRepo.On("GetPackage", ctx, req.CatalogID, req.PackageName).
		Return(domain.Package{}, domain.ErrNotFound)

	_, err := uc.Start(ctx, req)

	assert.ErrorIs(t, err, domain.ErrNotFound)
	m.secrets.AssertNotCalled(t, "EnsureOnyxiaSecret")
	m.helm.AssertNotCalled(t, "StartInstall")
}

// ❌ EnsureOnyxiaSecret fails → error propagated, Helm not called.
func TestStart_SecretError(t *testing.T) {
	uc, ctx, m := setupServiceLifecycle(t)
	req := baseRequest()
	pkg := resolvedPkg(req)

	m.pkgRepo.On("GetPackage", mock.Anything, mock.Anything, mock.Anything).
		Return(pkg, nil)
	m.secrets.On("EnsureOnyxiaSecret", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("k8s unavailable"))

	_, err := uc.Start(ctx, req)

	assert.ErrorContains(t, err, "k8s unavailable")
	m.helm.AssertNotCalled(t, "StartInstall")
}

// ❌ StartInstall fails (preflight error) → error propagated.
func TestStart_HelmError(t *testing.T) {
	uc, ctx, m := setupServiceLifecycle(t)
	req := baseRequest()
	pkg := resolvedPkg(req)

	m.pkgRepo.On("GetPackage", mock.Anything, mock.Anything, mock.Anything).
		Return(pkg, nil)
	m.secrets.On("EnsureOnyxiaSecret", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	m.helm.On("StartInstall", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("invalid release name"))

	_, err := uc.Start(ctx, req)

	assert.ErrorContains(t, err, "invalid release name")
}
