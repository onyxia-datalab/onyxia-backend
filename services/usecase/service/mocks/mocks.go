package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
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

type MockServiceRecordGateway struct{ mock.Mock }

var _ ports.ServiceRecordGateway = (*MockServiceRecordGateway)(nil)

func (m *MockServiceRecordGateway) CreateServiceRecord(
	ctx context.Context,
	namespace string,
	rec ports.ServiceRecord,
) error {
	return m.Called(ctx, namespace, rec).Error(0)
}

func (m *MockServiceRecordGateway) GetServiceRecord(
	ctx context.Context,
	namespace, releaseID string,
) (ports.ServiceRecord, error) {
	args := m.Called(ctx, namespace, releaseID)
	return args.Get(0).(ports.ServiceRecord), args.Error(1)
}

func (m *MockServiceRecordGateway) ListServiceRecords(
	ctx context.Context,
	namespace string,
) ([]ports.ServiceRecord, error) {
	args := m.Called(ctx, namespace)
	if v := args.Get(0); v != nil {
		return v.([]ports.ServiceRecord), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockServiceRecordGateway) SetServiceShared(
	ctx context.Context,
	namespace, releaseID string,
	share bool,
) error {
	return m.Called(ctx, namespace, releaseID, share).Error(0)
}

func (m *MockServiceRecordGateway) DeleteServiceRecord(ctx context.Context, namespace, releaseID string) error {
	return m.Called(ctx, namespace, releaseID).Error(0)
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

type MockCatalogService struct{ mock.Mock }

var _ ports.CatalogService = (*MockCatalogService)(nil)

func (m *MockCatalogService) ListPublicCatalogs(ctx context.Context) ([]domain.Catalog, error) {
	args := m.Called(ctx)
	if v := args.Get(0); v != nil {
		return v.([]domain.Catalog), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockCatalogService) ListUserCatalogs(ctx context.Context) ([]domain.Catalog, error) {
	args := m.Called(ctx)
	if v := args.Get(0); v != nil {
		return v.([]domain.Catalog), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockCatalogService) GetPackage(
	ctx context.Context,
	catalogID string,
	packageName string,
) (domain.Package, error) {
	args := m.Called(ctx, catalogID, packageName)
	return args.Get(0).(domain.Package), args.Error(1)
}

func (m *MockCatalogService) GetAvailableVersions(
	ctx context.Context,
	catalogID string,
	packageName string,
) ([]string, error) {
	args := m.Called(ctx, catalogID, packageName)
	if v := args.Get(0); v != nil {
		return v.([]string), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockCatalogService) GetPackageSchema(
	ctx context.Context,
	catalogID string,
	packageName string,
	version string,
) ([]byte, error) {
	args := m.Called(ctx, catalogID, packageName, version)
	if v := args.Get(0); v != nil {
		return v.([]byte), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockCatalogService) CheckSharingAllowed(ctx context.Context, catalogID string) error {
	return m.Called(ctx, catalogID).Error(0)
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
