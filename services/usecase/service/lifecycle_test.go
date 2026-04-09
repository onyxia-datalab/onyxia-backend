package service

import (
	"context"
	"errors"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type lifecycleMocks struct {
	helm    *MockReleaseGateway
	secrets *MockOnyxiaSecretGateway
	pkgRepo *MockCatalogRepository
}

func setupLifecycle(t *testing.T) (*Lifecycle, context.Context, lifecycleMocks) {
	t.Helper()
	m := lifecycleMocks{
		helm:    new(MockReleaseGateway),
		secrets: new(MockOnyxiaSecretGateway),
		pkgRepo: new(MockCatalogRepository),
	}
	uc := NewLifecycle(m.secrets, m.helm, m.pkgRepo)
	return uc, context.Background(), m
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

func TestStart_Success(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()
	pkg := resolvedPkg(req)

	m.pkgRepo.On("GetPackage", ctx, req.CatalogID, req.PackageName).Return(pkg, nil)
	m.secrets.On("EnsureOnyxiaSecret", ctx, req.Namespace, req.ReleaseID, mock.Anything).Return(nil)
	m.helm.On("StartInstall", ctx, req.Namespace, req.Name, mock.Anything, req.Version, req.Values, mock.Anything).Return(nil)

	_, err := uc.Start(ctx, req)

	require.NoError(t, err)
	m.pkgRepo.AssertExpectations(t)
	m.secrets.AssertExpectations(t)
	m.helm.AssertExpectations(t)
}

func TestStart_SecretDataIsCorrect(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()
	req.Share = true
	pkg := resolvedPkg(req)

	m.pkgRepo.On("GetPackage", mock.Anything, mock.Anything, mock.Anything).Return(pkg, nil)
	m.secrets.On("EnsureOnyxiaSecret", ctx, req.Namespace, req.ReleaseID,
		map[string][]byte{
			"catalog":      []byte(req.CatalogID),
			"friendlyName": []byte(req.FriendlyName),
			"owner":        []byte(req.Username),
			"share":        []byte("true"),
		},
	).Return(nil)
	m.helm.On("StartInstall", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	_, err := uc.Start(ctx, req)

	require.NoError(t, err)
	m.secrets.AssertExpectations(t)
}

func TestStart_GetPackageError(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()

	m.pkgRepo.On("GetPackage", ctx, req.CatalogID, req.PackageName).Return(domain.Package{}, errors.New("index unavailable"))

	_, err := uc.Start(ctx, req)

	assert.ErrorContains(t, err, "index unavailable")
	m.secrets.AssertNotCalled(t, "EnsureOnyxiaSecret")
	m.helm.AssertNotCalled(t, "StartInstall")
}

func TestStart_PackageNotFound(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()

	m.pkgRepo.On("GetPackage", ctx, req.CatalogID, req.PackageName).Return(domain.Package{}, domain.ErrNotFound)

	_, err := uc.Start(ctx, req)

	assert.ErrorIs(t, err, domain.ErrNotFound)
	m.secrets.AssertNotCalled(t, "EnsureOnyxiaSecret")
	m.helm.AssertNotCalled(t, "StartInstall")
}

func TestStart_SecretError(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()
	pkg := resolvedPkg(req)

	m.pkgRepo.On("GetPackage", mock.Anything, mock.Anything, mock.Anything).Return(pkg, nil)
	m.secrets.On("EnsureOnyxiaSecret", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("k8s unavailable"))

	_, err := uc.Start(ctx, req)

	assert.ErrorContains(t, err, "k8s unavailable")
	m.helm.AssertNotCalled(t, "StartInstall")
}

func TestStart_HelmError(t *testing.T) {
	uc, ctx, m := setupLifecycle(t)
	req := baseRequest()
	pkg := resolvedPkg(req)

	m.pkgRepo.On("GetPackage", mock.Anything, mock.Anything, mock.Anything).Return(pkg, nil)
	m.secrets.On("EnsureOnyxiaSecret", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	m.helm.On("StartInstall", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("invalid release name"))

	_, err := uc.Start(ctx, req)

	assert.ErrorContains(t, err, "invalid release name")
}
