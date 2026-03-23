package usecase

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/stretchr/testify/mock"
)

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
