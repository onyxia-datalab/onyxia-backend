package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

	uc := NewCatalogService(cfgs, env.SchemasConfig{}, repo, reader)
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

// ❌ User doesn't match any restriction — no catalogs returned.
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

	expected := domain.Package{Name: "my-chart", CatalogID: "my-catalog"}
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
	assert.Equal(t, domain.Package{}, result)
}

// ❌ GetPackage — repo returns an error.
func TestGetPackage_RepoError(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog"}}
	uc, ctx, repo := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	repo.On("GetPackage", mock.Anything, cfgs[0].ID, "my-chart").
		Return(domain.Package{}, errors.New("network failure"))

	result, err := uc.GetPackage(ctx, "my-catalog", "my-chart")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "network failure")
	assert.Equal(t, domain.Package{}, result)
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

// ✅ GetPackageSchema applies instance-wide override.
func TestGetPackageSchema_InstanceOverride(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog"}}
	schemasConfig := env.SchemasConfig{
		Enabled: true,
		Files: []env.SchemaFile{
			{
				RelativePath: "ide/customImage.json",
				Content:      `{"type":"string","const":"overridden"}`,
			},
		},
	}
	ctx, reader, _ := usercontext.NewTestUserContext(usercontext.DefaultTestUser())
	repo := new(MockCatalogRepository)
	uc := NewCatalogService(cfgs, schemasConfig, repo, reader)

	raw := []byte(`{"x-onyxia":{"overwriteSchemaWith":"ide/customImage.json"}}`)
	repo.On("GetPackageSchema", mock.Anything, cfgs[0].ID, "my-chart", "1.0.0").
		Return(raw, nil)

	result, err := uc.GetPackageSchema(ctx, "my-catalog", "my-chart", "1.0.0")

	assert.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(result, &got))
	assert.Equal(t, "overridden", got["const"])
}

// ✅ GetPackageSchema applies role-specific override over instance-wide.
func TestGetPackageSchema_RoleOverrideTakesPrecedenceOverInstance(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog"}}
	schemasConfig := env.SchemasConfig{
		Enabled: true,
		Files: []env.SchemaFile{
			{RelativePath: "ide/resources.json", Content: `{"title":"instance-override"}`},
		},
		Roles: []env.RoleSchemas{
			{
				RoleName: "fullgpu",
				Files: []env.SchemaFile{
					{RelativePath: "ide/resources.json", Content: `{"title":"role-override"}`},
				},
			},
		},
	}
	user := &usercontext.User{
		Username: "gpu-user",
		Roles:    []string{"fullgpu"},
	}
	ctx, reader, _ := usercontext.NewTestUserContext(user)
	repo := new(MockCatalogRepository)
	uc := NewCatalogService(cfgs, schemasConfig, repo, reader)

	raw := []byte(`{"x-onyxia":{"overwriteSchemaWith":"ide/resources.json"}}`)
	repo.On("GetPackageSchema", mock.Anything, cfgs[0].ID, "my-chart", "1.0.0").
		Return(raw, nil)

	result, err := uc.GetPackageSchema(ctx, "my-catalog", "my-chart", "1.0.0")

	assert.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(result, &got))
	assert.Equal(t, "role-override", got["title"])
}

// ✅ GetPackageSchema leaves unknown overwriteSchemaWith paths unchanged.
func TestGetPackageSchema_UnknownPathLeftUnchanged(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog"}}
	uc, ctx, repo := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	raw := []byte(`{"x-onyxia":{"overwriteSchemaWith":"unknown/path.json"}}`)
	repo.On("GetPackageSchema", mock.Anything, cfgs[0].ID, "my-chart", "1.0.0").
		Return(raw, nil)

	result, err := uc.GetPackageSchema(ctx, "my-catalog", "my-chart", "1.0.0")

	assert.NoError(t, err)
	// Node must remain unchanged since the path is unknown.
	var got map[string]any
	require.NoError(t, json.Unmarshal(result, &got))
	xOnyxia := got["x-onyxia"].(map[string]any)
	assert.Equal(t, "unknown/path.json", xOnyxia["overwriteSchemaWith"])
}

// ❌ GetPackageSchema — package is excluded.
func TestGetPackageSchema_ExcludedPackage(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog", Excluded: []string{"excluded-chart"}}}
	uc, ctx, _ := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	result, err := uc.GetPackageSchema(ctx, "my-catalog", "excluded-chart", "1.0.0")

	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Nil(t, result)
}

// ❌ GetPackage — package is excluded.
func TestGetPackage_ExcludedPackage(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog", Excluded: []string{"excluded-chart"}}}
	uc, ctx, _ := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	result, err := uc.GetPackage(ctx, "my-catalog", "excluded-chart")

	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, domain.Package{}, result)
}

// ❌ GetAvailableVersions — catalog not found.
func TestGetAvailableVersions_CatalogNotFound(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog"}}
	uc, ctx, _ := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	_, err := uc.GetAvailableVersions(ctx, "unknown-catalog", "my-chart")

	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ❌ GetAvailableVersions — package is excluded.
func TestGetAvailableVersions_ExcludedPackage(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog", Excluded: []string{"excluded-chart"}}}
	uc, ctx, _ := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	_, err := uc.GetAvailableVersions(ctx, "my-catalog", "excluded-chart")

	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ❌ GetAvailableVersions — repo returns an error.
func TestGetAvailableVersions_RepoError(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "my-catalog"}}
	uc, ctx, repo := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	repo.On("GetAvailableVersions", mock.Anything, "my-catalog", "my-chart").
		Return(nil, errors.New("index unavailable"))

	_, err := uc.GetAvailableVersions(ctx, "my-catalog", "my-chart")

	assert.ErrorContains(t, err, "index unavailable")
}

// ❌ GetAvailableVersions — maxNumber mode without maxNumberOfVersions set.
func TestGetAvailableVersions_InvalidMaxNumber(t *testing.T) {
	cfgs := []env.CatalogConfig{{
		ID:                   "my-catalog",
		MultipleServicesMode: env.MultipleServicesMaxNumber,
		MaxNumberOfVersions:  nil,
	}}
	uc, ctx, repo := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	repo.On("GetAvailableVersions", mock.Anything, "my-catalog", "my-chart").
		Return([]string{"1.0.0"}, nil)

	_, err := uc.GetAvailableVersions(ctx, "my-catalog", "my-chart")

	assert.ErrorContains(t, err, "maxNumberOfVersions")
}

// ✅ ListUserCatalogs — unrestricted catalog is always included.
func TestListUserCatalogs_UnrestrictedIncluded(t *testing.T) {
	cfgs := []env.CatalogConfig{{ID: "public"}}
	uc, ctx, repo := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	repo.On("ListPackages", mock.Anything, "public").Return([]domain.Package{}, nil)

	result, err := uc.ListUserCatalogs(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "public", result[0].ID)
}

// ❌ ListUserCatalogs — user has no attributes, restricted catalog excluded.
func TestListUserCatalogs_NoAttributes(t *testing.T) {
	user := &usercontext.User{Username: "anon"}
	cfgs := []env.CatalogConfig{{
		ID:           "restricted",
		Restrictions: []env.Restriction{{UserAttributeKey: "groups", Match: "admin"}},
	}}
	uc, ctx, _ := setupCatalogUsecase(t, user, cfgs)

	result, err := uc.ListUserCatalogs(ctx)

	assert.NoError(t, err)
	assert.Empty(t, result)
}

// ❌ ListUserCatalogs — restrictions with empty key/match are skipped, no match found.
func TestListUserCatalogs_SkipsEmptyAndInvalidRestrictions(t *testing.T) {
	user := &usercontext.User{
		Username:   "alice",
		Attributes: map[string]any{"groups": []string{"users"}},
	}
	cfgs := []env.CatalogConfig{{
		ID: "restricted",
		Restrictions: []env.Restriction{
			{UserAttributeKey: "", Match: "admin"},          // empty key → skip
			{UserAttributeKey: "groups", Match: ""},         // empty match → skip
			{UserAttributeKey: "missing-key", Match: ".*"},  // key absent → skip
			{UserAttributeKey: "groups", Match: "[invalid"}, // bad regex → skip
		},
	}}
	uc, ctx, _ := setupCatalogUsecase(t, user, cfgs)

	result, err := uc.ListUserCatalogs(ctx)

	assert.NoError(t, err)
	assert.Empty(t, result)
}

// ✅ ListUserCatalogs — string attribute matched by restriction.
func TestListUserCatalogs_StringAttribute(t *testing.T) {
	user := &usercontext.User{
		Username:   "alice",
		Attributes: map[string]any{"role": "admin"},
	}
	cfgs := []env.CatalogConfig{{
		ID:           "admin-catalog",
		Restrictions: []env.Restriction{{UserAttributeKey: "role", Match: "admin"}},
	}}
	uc, ctx, repo := setupCatalogUsecase(t, user, cfgs)
	repo.On("ListPackages", mock.Anything, "admin-catalog").Return([]domain.Package{}, nil)

	result, err := uc.ListUserCatalogs(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

// ✅ ListUserCatalogs — []any attribute matched by restriction.
func TestListUserCatalogs_AnySliceAttribute(t *testing.T) {
	user := &usercontext.User{
		Username:   "alice",
		Attributes: map[string]any{"groups": []any{"sspcloud-dev", "users"}},
	}
	cfgs := []env.CatalogConfig{{
		ID:           "dev-catalog",
		Restrictions: []env.Restriction{{UserAttributeKey: "groups", Match: "sspcloud-dev"}},
	}}
	uc, ctx, repo := setupCatalogUsecase(t, user, cfgs)
	repo.On("ListPackages", mock.Anything, "dev-catalog").Return([]domain.Package{}, nil)

	result, err := uc.ListUserCatalogs(ctx)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
}

// ✅ GetAvailableVersions applies MaxNumber filter.
func TestGetAvailableVersions_MaxNumber(t *testing.T) {
	n := 2
	cfgs := []env.CatalogConfig{{
		ID:                   "my-catalog",
		MultipleServicesMode: env.MultipleServicesMaxNumber,
		MaxNumberOfVersions:  &n,
	}}
	uc, ctx, repo := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	repo.On("GetAvailableVersions", mock.Anything, cfgs[0].ID, "my-chart").
		Return([]string{"3.0.0", "2.0.0", "1.0.0"}, nil)

	versions, err := uc.GetAvailableVersions(ctx, "my-catalog", "my-chart")

	assert.NoError(t, err)
	assert.Equal(t, []string{"3.0.0", "2.0.0"}, versions)
}

// ✅ GetAvailableVersions applies SkipPatches filter.
func TestGetAvailableVersions_SkipPatches(t *testing.T) {
	cfgs := []env.CatalogConfig{{
		ID:                   "my-catalog",
		MultipleServicesMode: env.MultipleServicesSkipPatches,
	}}
	uc, ctx, repo := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	repo.On("GetAvailableVersions", mock.Anything, cfgs[0].ID, "my-chart").
		Return([]string{"2.1.1", "2.1.0", "1.0.5", "1.0.0"}, nil)

	versions, err := uc.GetAvailableVersions(ctx, "my-catalog", "my-chart")

	assert.NoError(t, err)
	assert.Equal(t, []string{"2.1.1", "1.0.5"}, versions)
}

// ✅ GetAvailableVersions applies Latest filter.
func TestGetAvailableVersions_Latest(t *testing.T) {
	cfgs := []env.CatalogConfig{{
		ID:                   "my-catalog",
		MultipleServicesMode: env.MultipleServicesLatest,
	}}
	uc, ctx, repo := setupCatalogUsecase(t, usercontext.DefaultTestUser(), cfgs)

	repo.On("GetAvailableVersions", mock.Anything, cfgs[0].ID, "my-chart").
		Return([]string{"2.0.0", "1.0.0"}, nil)

	versions, err := uc.GetAvailableVersions(ctx, "my-catalog", "my-chart")

	assert.NoError(t, err)
	assert.Equal(t, []string{"2.0.0"}, versions)
}
