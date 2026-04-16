package mocks

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/stretchr/testify/mock"
)

type MockReleaseGateway struct{ mock.Mock }

var _ ports.ReleaseGateway = (*MockReleaseGateway)(nil)

func (m *MockReleaseGateway) StartInstall(
	ctx context.Context,
	namespace string,
	releaseName string,
	pkg *domain.Package,
	version string,
	vals map[string]interface{},
	opts ports.InstallOptions,
) error {
	return m.Called(ctx, namespace, releaseName, pkg, version, vals, opts).Error(0)
}

func (m *MockReleaseGateway) SuspendRelease(ctx context.Context, namespace, releaseName string) error {
	return m.Called(ctx, namespace, releaseName).Error(0)
}

func (m *MockReleaseGateway) ResumeRelease(ctx context.Context, namespace, releaseName string) error {
	return m.Called(ctx, namespace, releaseName).Error(0)
}

func (m *MockReleaseGateway) UninstallRelease(ctx context.Context, namespace, releaseName string) error {
	return m.Called(ctx, namespace, releaseName).Error(0)
}

func (m *MockReleaseGateway) GetReleaseState(
	ctx context.Context,
	namespace, releaseName string,
) (ports.ReleaseState, error) {
	args := m.Called(ctx, namespace, releaseName)
	return args.Get(0).(ports.ReleaseState), args.Error(1)
}

func (m *MockReleaseGateway) GetReleaseResources(
	ctx context.Context,
	namespace, releaseName string,
) ([]ports.ManifestResource, error) {
	args := m.Called(ctx, namespace, releaseName)
	if v := args.Get(0); v != nil {
		return v.([]ports.ManifestResource), args.Error(1)
	}
	return nil, args.Error(1)
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

func (m *MockOnyxiaSecretGateway) ListOnyxiaSecretNames(
	ctx context.Context,
	namespace string,
) ([]string, error) {
	args := m.Called(ctx, namespace)
	if v := args.Get(0); v != nil {
		return v.([]string), args.Error(1)
	}
	return nil, args.Error(1)
}

type MockCatalogRepository struct{ mock.Mock }

var _ ports.PackageRepository = (*MockCatalogRepository)(nil)

func (m *MockCatalogRepository) ListPackages(
	ctx context.Context,
	catalogID string,
) ([]domain.Package, error) {
	args := m.Called(ctx, catalogID)
	val := args.Get(0)
	if val == nil {
		return nil, args.Error(1)
	}
	return val.([]domain.Package), args.Error(1)
}

func (m *MockCatalogRepository) GetPackage(
	ctx context.Context,
	catalogID string,
	name string,
) (domain.Package, error) {
	args := m.Called(ctx, catalogID, name)
	return args.Get(0).(domain.Package), args.Error(1)
}

func (m *MockCatalogRepository) GetAvailableVersions(
	ctx context.Context,
	catalogID string,
	name string,
) ([]string, error) {
	args := m.Called(ctx, catalogID, name)
	if res := args.Get(0); res != nil {
		return res.([]string), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockCatalogRepository) GetPackageSchema(
	ctx context.Context,
	catalogID string,
	packageName string,
	version string,
) ([]byte, error) {
	args := m.Called(ctx, catalogID, packageName, version)
	if res := args.Get(0); res != nil {
		return res.([]byte), args.Error(1)
	}
	return nil, args.Error(1)
}

type MockWorkloadStateGateway struct{ mock.Mock }

var _ ports.WorkloadStateGateway = (*MockWorkloadStateGateway)(nil)

func (m *MockWorkloadStateGateway) GetPodsForRelease(
	ctx context.Context,
	namespace, releaseID string,
) ([]ports.PodInfo, error) {
	args := m.Called(ctx, namespace, releaseID)
	if v := args.Get(0); v != nil {
		return v.([]ports.PodInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWorkloadStateGateway) GetControllerReadiness(
	ctx context.Context,
	namespace string,
	resources []ports.ManifestResource,
) (bool, error) {
	args := m.Called(ctx, namespace, resources)
	return args.Bool(0), args.Error(1)
}
