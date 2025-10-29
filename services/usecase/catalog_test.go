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

const (
	testUserName = "test-user"
	testGroup    = "sspcloud-dev"
)

type MockCatalogRepository struct{ mock.Mock }

var _ ports.CatalogRepository = (*MockCatalogRepository)(nil)

func (m *MockCatalogRepository) ListPackages(
	ctx context.Context,
	cfg env.CatalogConfig,
) ([]domain.Package, error) {
	args := m.Called(ctx, cfg)
	val := args.Get(0)
	if val == nil {
		return nil, args.Error(1)
	}
	return val.([]domain.Package), args.Error(1)
}

func (m *MockCatalogRepository) GetPackage(
	ctx context.Context,
	cfg env.CatalogConfig,
	name string,
) (*domain.PackageRef, error) {
	args := m.Called(ctx, cfg, name)
	if res := args.Get(0); res != nil {
		return res.(*domain.PackageRef), args.Error(1)
	}
	return nil, args.Error(1)
}

// ---------- Setup Helper ----------
func setupCatalogUsecase(cfgs []env.CatalogConfig, repo ports.CatalogRepository) *Catalog {
	return NewCatalogService(cfgs, repo)
}

// ---------- Tests ----------

// ✅ Test public catalogs (no restrictions)
func TestListPublicCatalogs(t *testing.T) {
	repo := new(MockCatalogRepository)
	repo.On("ListPackages", mock.Anything, mock.AnythingOfType("env.CatalogConfig")).
		Return([]domain.Package{{Name: "chart"}}, nil)

	cfgs := []env.CatalogConfig{
		{ID: "public", Restrictions: nil},
		{
			ID: "restricted",
			Restrictions: []env.Restriction{
				{UserAttributeKey: "groups", Match: "sspcloud-dev"},
			},
		},
	}

	usecase := setupCatalogUsecase(cfgs, repo)

	catalogs, err := usecase.ListPublicCatalogs(context.Background())
	assert.NoError(t, err)
	assert.Len(t, catalogs, 1)
	assert.Equal(t, "public", catalogs[0].ID)

	repo.AssertCalled(t, "ListPackages", mock.Anything, cfgs[0])
	repo.AssertNotCalled(t, "ListPackages", mock.Anything, cfgs[1])
}

// ✅ Test restricted catalog accessible by user
func TestListUserCatalog_Match(t *testing.T) {
	repo := new(MockCatalogRepository)
	repo.On("ListPackages", mock.Anything, mock.AnythingOfType("env.CatalogConfig")).
		Return([]domain.Package{{Name: "chart"}}, nil)

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

	// ✅ use built-in test context
	testUser := &usercontext.User{
		Username: "test-user",
		Groups:   []string{"sspcloud-dev", "users"},
		Roles:    []string{"role1"},
		Attributes: map[string]any{
			"groups": []string{"sspcloud-dev", "users"},
		},
	}
	ctx, reader, _ := usercontext.NewTestUserContext(testUser)

	usecase := setupCatalogUsecase(cfgs, repo)
	result, err := usecase.ListUserCatalog(ctx, reader)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "restricted-dev", result[0].ID)
	repo.AssertCalled(t, "ListPackages", mock.Anything, cfgs[0])
}

// ❌ Test restricted catalog with no matching user attributes
func TestListUserCatalog_NoMatch(t *testing.T) {
	repo := new(MockCatalogRepository)
	repo.On("ListPackages", mock.Anything, mock.AnythingOfType("env.CatalogConfig")).
		Return([]domain.Package{{Name: "chart"}}, nil)

	cfgs := []env.CatalogConfig{
		{
			ID: "restricted-admin",
			Restrictions: []env.Restriction{
				{UserAttributeKey: "groups", Match: "sspcloud-admin"},
			},
		},
	}

	testUser := &usercontext.User{
		Username: "test-user",
		Groups:   []string{"sspcloud-guest"},
		Roles:    []string{"role1"},
		Attributes: map[string]any{
			"groups": []string{"sspcloud-guest"},
		},
	}
	ctx, reader, _ := usercontext.NewTestUserContext(testUser)

	usecase := setupCatalogUsecase(cfgs, repo)
	result, err := usecase.ListUserCatalog(ctx, reader)

	assert.NoError(t, err)
	assert.Len(t, result, 0)
	repo.AssertNotCalled(t, "ListPackages", mock.Anything, cfgs[0])
}

// ❌ Test repo returns an error
func TestListUserCatalog_RepoError(t *testing.T) {
	repo := new(MockCatalogRepository)
	repo.On("ListPackages", mock.Anything, mock.AnythingOfType("env.CatalogConfig")).
		Return(nil, errors.New("failed to fetch"))

	cfgs := []env.CatalogConfig{
		{
			ID: "restricted-dev",
			Restrictions: []env.Restriction{
				{UserAttributeKey: "groups", Match: "sspcloud-dev"},
			},
		},
	}

	testUser := &usercontext.User{
		Username: "test-user",
		Groups:   []string{"sspcloud-dev"},
		Attributes: map[string]any{
			"groups": []string{"sspcloud-dev"},
		},
	}
	ctx, reader, _ := usercontext.NewTestUserContext(testUser)

	usecase := setupCatalogUsecase(cfgs, repo)
	result, err := usecase.ListUserCatalog(ctx, reader)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to fetch")
	repo.AssertCalled(t, "ListPackages", mock.Anything, cfgs[0])
}
