package controller

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/internal/tools"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	api "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type catalogStub struct {
	catalogs    []domain.Catalog
	listErr     error
	pkg         domain.Package
	pkgErr      error
	versions    []string
	versionsErr error
	schema      []byte
	schemaErr   error

	gotUser *usercontext.User
}

func (s *catalogStub) ListCatalogs(_ context.Context, user *usercontext.User) ([]domain.Catalog, error) {
	s.gotUser = user
	return s.catalogs, s.listErr
}

func (s *catalogStub) GetPackage(
	_ context.Context,
	user *usercontext.User,
	_, _ string,
) (domain.Package, error) {
	s.gotUser = user
	return s.pkg, s.pkgErr
}

func (s *catalogStub) GetAvailableVersions(
	context.Context,
	*usercontext.User,
	string, string,
) ([]string, error) {
	return s.versions, s.versionsErr
}

func (s *catalogStub) GetPackageSchema(
	_ context.Context,
	user *usercontext.User,
	_, _, _ string,
) ([]byte, error) {
	s.gotUser = user
	return s.schema, s.schemaErr
}

func (s *catalogStub) CheckSharingAllowed(context.Context, *usercontext.User, string) error {
	return nil
}

func mustLocalized(t *testing.T, v any) tools.LocalizedString {
	t.Helper()
	s, err := tools.NewLocalizedString(v)
	require.NoError(t, err)
	return s
}

func TestGetMyCatalogsMapsCatalogs(t *testing.T) {
	home, _ := url.Parse("https://jupyter.org")
	icon, _ := url.Parse("https://jupyter.org/icon.svg")
	stub := &catalogStub{catalogs: []domain.Catalog{
		{
			ID:                  "ide",
			Name:                mustLocalized(t, "IDE"),
			Description:         mustLocalized(t, "Development environments"),
			Status:              domain.CatalogStatusProd,
			HighlightedPackages: []string{"jupyter"},
			Packages: []domain.Package{
				{Name: "jupyter", Description: "Notebooks", HomeUrl: *home, IconUrl: *icon},
			},
		},
		{
			ID:          "db",
			Name:        mustLocalized(t, map[string]string{"en": "Databases", "fr": "Bases de données"}),
			Description: mustLocalized(t, map[string]string{"en": "Storage"}),
			Status:      domain.CatalogStatusTest,
		},
		{
			ID:     "odd",
			Name:   mustLocalized(t, "Odd"),
			Status: "BETA",
		},
	}}
	ctx, users, _ := usercontext.NewTestUserContext(&usercontext.User{Username: "alice"})
	ctrl := NewCatalogController(stub, users)

	res, err := ctrl.GetMyCatalogs(ctx)

	require.NoError(t, err)
	require.NotNil(t, stub.gotUser)
	assert.Equal(t, "alice", stub.gotUser.Username)

	catalogs := *res.(*api.GetMyCatalogsOKApplicationJSON)
	require.Len(t, catalogs, 3)

	ide := catalogs[0]
	assert.Equal(t, "ide", ide.ID)
	name, ok := ide.Name.GetString()
	require.True(t, ok)
	assert.Equal(t, "IDE", name)
	desc, ok := ide.Description.Get()
	require.True(t, ok)
	plainDesc, ok := desc.GetString()
	require.True(t, ok)
	assert.Equal(t, "Development environments", plainDesc)
	assert.Equal(t, api.NewOptCatalogStatus(api.CatalogStatusPROD), ide.Status)
	assert.Equal(t, []string{"jupyter"}, ide.HighlightedPackages)
	require.Len(t, ide.Packages, 1)
	assert.Equal(t, api.Package{
		Name:        "jupyter",
		Description: api.NewOptString("Notebooks"),
		Icon:        *icon,
		Home:        api.NewOptURI(*home),
	}, ide.Packages[0])

	db := catalogs[1]
	multiName, ok := db.Name.GetLocalizedString1()
	require.True(t, ok)
	assert.Equal(t, "Bases de données", multiName["fr"])
	desc, ok = db.Description.Get()
	require.True(t, ok)
	multiDesc, ok := desc.GetLocalizedString1()
	require.True(t, ok)
	assert.Equal(t, "Storage", multiDesc["en"])
	assert.Equal(t, api.NewOptCatalogStatus(api.CatalogStatusTEST), db.Status)
	assert.Empty(t, db.Packages)

	// An unknown status is left out rather than sent to the client.
	assert.False(t, catalogs[2].Status.IsSet())
}

func TestGetMyCatalogsAnonymous(t *testing.T) {
	stub := &catalogStub{}
	users, _ := usercontext.NewUserContext()
	ctrl := NewCatalogController(stub, users)

	res, err := ctrl.GetMyCatalogs(context.Background())

	require.NoError(t, err)
	assert.Nil(t, stub.gotUser)
	assert.Empty(t, *res.(*api.GetMyCatalogsOKApplicationJSON))
}

func TestGetMyCatalogsPropagatesError(t *testing.T) {
	boom := errors.New("boom")
	users, _ := usercontext.NewUserContext()
	ctrl := NewCatalogController(&catalogStub{listErr: boom}, users)

	res, err := ctrl.GetMyCatalogs(context.Background())

	assert.Nil(t, res)
	assert.ErrorIs(t, err, boom)
}

func TestGetMyPackage(t *testing.T) {
	home, _ := url.Parse("https://jupyter.org")
	stub := &catalogStub{
		pkg:      domain.Package{Name: "jupyter", Description: "Notebooks", HomeUrl: *home},
		versions: []string{"2.0.0", "1.0.0"},
	}
	ctx, users, _ := usercontext.NewTestUserContext(&usercontext.User{Username: "alice"})
	ctrl := NewCatalogController(stub, users)

	res, err := ctrl.GetMyPackage(ctx, "ide", "jupyter")

	require.NoError(t, err)
	assert.Equal(t, &api.DetailedPackage{
		Name:        "jupyter",
		Description: api.NewOptString("Notebooks"),
		Home:        api.NewOptURI(*home),
		Versions:    []string{"2.0.0", "1.0.0"},
	}, res)
	assert.Equal(t, "alice", stub.gotUser.Username)
}

func TestGetMyPackageErrors(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name string
		stub *catalogStub
		want error
	}{
		{name: "not found", stub: &catalogStub{pkgErr: domain.ErrNotFound}, want: domain.ErrNotFound},
		{name: "package failure", stub: &catalogStub{pkgErr: boom}, want: boom},
		{name: "versions failure", stub: &catalogStub{versionsErr: boom}, want: boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users, _ := usercontext.NewUserContext()
			ctrl := NewCatalogController(tt.stub, users)

			res, err := ctrl.GetMyPackage(context.Background(), "ide", "jupyter")

			assert.Nil(t, res)
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestGetPackageSchema(t *testing.T) {
	stub := &catalogStub{schema: []byte(`{"type":"object","properties":{"a":{"type":"string"}}}`)}
	users, _ := usercontext.NewUserContext()
	ctrl := NewCatalogController(stub, users)

	res, err := ctrl.GetPackageSchema(context.Background(), "ide", "jupyter", "1.0.0")

	require.NoError(t, err)
	schema := *res.(*api.GetPackageSchemaOK)
	require.Len(t, schema, 2)
	assert.JSONEq(t, `"object"`, string(schema["type"]))
	assert.JSONEq(t, `{"a":{"type":"string"}}`, string(schema["properties"]))
}

func TestGetPackageSchemaErrors(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name    string
		stub    *catalogStub
		wantErr error
	}{
		{name: "not found", stub: &catalogStub{schemaErr: domain.ErrNotFound}, wantErr: domain.ErrNotFound},
		{name: "failure", stub: &catalogStub{schemaErr: boom}, wantErr: boom},
		{name: "not a JSON object", stub: &catalogStub{schema: []byte(`[1,2]`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users, _ := usercontext.NewUserContext()
			ctrl := NewCatalogController(tt.stub, users)

			res, err := ctrl.GetPackageSchema(context.Background(), "ide", "jupyter", "1.0.0")

			assert.Nil(t, res)
			require.Error(t, err)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}
