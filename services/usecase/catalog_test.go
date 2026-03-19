package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ---------- Mock Repository ----------

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
) (*domain.PackageRef, error) {
	args := m.Called(ctx, catalogID, name)
	if res := args.Get(0); res != nil {
		return res.(*domain.PackageRef), args.Error(1)
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

func (m *MockCatalogRepository) ResolvePackage(
	ctx context.Context,
	catalogID string,
	packageName string,
	version string,
) (domain.PackageVersion, error) {
	args := m.Called(ctx, catalogID, packageName, version)
	if res := args.Get(0); res != nil {
		return res.(domain.PackageVersion), args.Error(1)
	}
	return domain.PackageVersion{}, args.Error(1)
}

// ---------- Setup Helper ----------

// setupCatalogUsecase abstracts away mocks, readers, and context initialization.
func setupCatalogUsecase(
	t *testing.T,
	user *usercontext.User,
	cfgs []env.CatalogConfig,
) (*Catalog, context.Context, *MockCatalogRepository) {
	t.Helper()

	ctx, reader, _ := usercontext.NewTestUserContext(user)
	repo := new(MockCatalogRepository)

	if len(cfgs) == 0 {
		cfgs = []env.CatalogConfig{
			{
				ID: "default-catalog",
				Restrictions: []env.Restriction{
					{UserAttributeKey: "groups", Match: "sspcloud-(dev|admin)"},
				},
			},
		}
	}

	uc := NewCatalogService(cfgs, repo, reader)
	return uc, ctx, repo
}

// ---------- Tests ----------

// ✅ Public catalogs should only include unrestricted ones.
func TestListPublicCatalogs(t *testing.T) {
	user := usercontext.DefaultTestUser()
	cfgs := []env.CatalogConfig{
		{ID: "public"},
		{
			ID: "restricted",
			Restrictions: []env.Restriction{
				{UserAttributeKey: "groups", Match: "sspcloud-dev"},
			},
		},
	}

	uc, ctx, repo := setupCatalogUsecase(t, user, cfgs)
	repo.On("ListPackages", mock.Anything, cfgs[0].ID).
		Return([]domain.Package{{Name: "chart"}}, nil)

	catalogs, err := uc.ListPublicCatalogs(ctx)

	assert.NoError(t, err)
	assert.Len(t, catalogs, 1)
	assert.Equal(t, "public", catalogs[0].ID)
	repo.AssertCalled(t, "ListPackages", mock.Anything, cfgs[0].ID)
	repo.AssertNotCalled(t, "ListPackages", mock.Anything, cfgs[1].ID)
}

// ✅ User has access to matching restricted catalog.
func TestListUserCatalogs_Match(t *testing.T) {
	user := &usercontext.User{
		Username: "test-user",
		Groups:   []string{"sspcloud-dev", "users"},
		Roles:    []string{"role1"},
		Attributes: map[string]any{
			"groups": []string{"sspcloud-dev", "users"},
		},
	}
	cfgs := []env.CatalogConfig{
		{
			ID: "restricted-dev",
			Restrictions: []env.Restriction{
				{UserAttributeKey: "groups", Match: "sspcloud-(dev|admin)"},
			},
		},
		{
			ID: "restricted-ops",
			Restrictions: []env.Restriction{
				{UserAttributeKey: "groups", Match: "sspcloud-ops"},
			},
		},
	}

	uc, ctx, repo := setupCatalogUsecase(t, user, cfgs)
	repo.On("ListPackages", mock.Anything, cfgs[0].ID).
		Return([]domain.Package{{Name: "chart"}}, nil)

	result, err := uc.ListUserCatalogs(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "restricted-dev", result[0].ID)
	repo.AssertCalled(t, "ListPackages", mock.Anything, cfgs[0].ID)
	repo.AssertNotCalled(t, "ListPackages", mock.Anything, cfgs[1].ID)
}

// ❌ User doesn’t match any restriction — no catalogs returned.
func TestListUserCatalogs_NoMatch(t *testing.T) {
	user := &usercontext.User{
		Username: "guest",
		Groups:   []string{"sspcloud-guest"},
		Attributes: map[string]any{
			"groups": []string{"sspcloud-guest"},
		},
	}
	cfgs := []env.CatalogConfig{
		{
			ID: "restricted-admin",
			Restrictions: []env.Restriction{
				{UserAttributeKey: "groups", Match: "sspcloud-admin"},
			},
		},
	}

	uc, ctx, repo := setupCatalogUsecase(t, user, cfgs)
	repo.On("ListPackages", mock.Anything, mock.Anything).
		Return([]domain.Package{{Name: "chart"}}, nil)

	result, err := uc.ListUserCatalogs(ctx)

	assert.NoError(t, err)
	assert.Empty(t, result)
	repo.AssertNotCalled(t, "ListPackages", mock.Anything, cfgs[0].ID)
}

// ❌ Repository returns an error — should propagate up.
func TestListUserCatalogs_RepoError(t *testing.T) {
	user := &usercontext.User{
		Username: "dev",
		Groups:   []string{"sspcloud-dev"},
		Attributes: map[string]any{
			"groups": []string{"sspcloud-dev"},
		},
	}
	cfgs := []env.CatalogConfig{
		{
			ID: "restricted-dev",
			Restrictions: []env.Restriction{
				{UserAttributeKey: "groups", Match: "sspcloud-dev"},
			},
		},
	}

	uc, ctx, repo := setupCatalogUsecase(t, user, cfgs)
	repo.On("ListPackages", mock.Anything, cfgs[0].ID).
		Return(nil, errors.New("failed to fetch"))

	result, err := uc.ListUserCatalogs(ctx)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to fetch")
	repo.AssertCalled(t, "ListPackages", mock.Anything, cfgs[0].ID)
}

// ✅ GetPackage returns the package when found.
func TestGetPackage_Found(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog"}}
	uc, ctx, repo := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	expected := &domain.PackageRef{
		Package:  domain.Package{Name: "my-chart", CatalogID: "my-catalog"},
		Versions: []string{"1.0.0", "0.9.0"},
	}
	repo.On("GetPackage", mock.Anything, cfgs[0].ID, "my-chart").Return(expected, nil)

	result, err := uc.GetPackage(ctx, "my-catalog", "my-chart")

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

// ❌ GetPackage — catalog not found.
func TestGetPackage_CatalogNotFound(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog"}}
	uc, ctx, _ := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	result, err := uc.GetPackage(ctx, "unknown-catalog", "my-chart")

	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Nil(t, result)
}

// ❌ GetPackage — repo returns an error.
func TestGetPackage_RepoError(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog"}}
	uc, ctx, repo := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	repo.On("GetPackage", mock.Anything, cfgs[0].ID, "my-chart").
		Return(nil, errors.New("network failure"))

	result, err := uc.GetPackage(ctx, "my-catalog", "my-chart")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network failure")
	assert.Nil(t, result)
}

// ✅ GetPackageSchema returns the schema bytes when found.
func TestGetPackageSchema_Found(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog"}}
	uc, ctx, repo := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	schema := []byte(`{"type":"object"}`)
	repo.On("GetPackageSchema", mock.Anything, cfgs[0].ID, "my-chart", "1.0.0").
		Return(schema, nil)

	result, err := uc.GetPackageSchema(ctx, "my-catalog", "my-chart", "1.0.0")

	assert.NoError(t, err)
	assert.Equal(t, schema, result)
}

// ❌ GetPackageSchema — catalog not found.
func TestGetPackageSchema_CatalogNotFound(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog"}}
	uc, ctx, _ := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	result, err := uc.GetPackageSchema(ctx, "unknown-catalog", "my-chart", "1.0.0")

	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Nil(t, result)
}

// ❌ GetPackageSchema — repo returns an error.
func TestGetPackageSchema_RepoError(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog"}}
	uc, ctx, repo := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	repo.On("GetPackageSchema", mock.Anything, cfgs[0].ID, "my-chart", "1.0.0").
		Return(nil, errors.New("schema fetch failed"))

	result, err := uc.GetPackageSchema(ctx, "my-catalog", "my-chart", "1.0.0")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema fetch failed")
	assert.Nil(t, result)
}
