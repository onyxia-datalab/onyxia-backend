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
	repo.On("ListPackages", mock.Anything, cfgs[0]).
		Return([]domain.Package{{Name: "chart"}}, nil)

	catalogs, err := uc.ListPublicCatalogs(ctx)

	assert.NoError(t, err)
	assert.Len(t, catalogs, 1)
	assert.Equal(t, "public", catalogs[0].ID)
	repo.AssertCalled(t, "ListPackages", mock.Anything, cfgs[0])
	repo.AssertNotCalled(t, "ListPackages", mock.Anything, cfgs[1])
}

// ✅ User has access to matching restricted catalog.
func TestListUserCatalog_Match(t *testing.T) {
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
	repo.On("ListPackages", mock.Anything, cfgs[0]).
		Return([]domain.Package{{Name: "chart"}}, nil)

	result, err := uc.ListUserCatalog(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "restricted-dev", result[0].ID)
	repo.AssertCalled(t, "ListPackages", mock.Anything, cfgs[0])
	repo.AssertNotCalled(t, "ListPackages", mock.Anything, cfgs[1])
}

// ❌ User doesn’t match any restriction — no catalogs returned.
func TestListUserCatalog_NoMatch(t *testing.T) {
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

	result, err := uc.ListUserCatalog(ctx)

	assert.NoError(t, err)
	assert.Empty(t, result)
	repo.AssertNotCalled(t, "ListPackages", mock.Anything, cfgs[0])
}

// ❌ Repository returns an error — should propagate up.
func TestListUserCatalog_RepoError(t *testing.T) {
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
	repo.On("ListPackages", mock.Anything, cfgs[0]).
		Return(nil, errors.New("failed to fetch"))

	result, err := uc.ListUserCatalog(ctx)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to fetch")
	repo.AssertCalled(t, "ListPackages", mock.Anything, cfgs[0])
}
