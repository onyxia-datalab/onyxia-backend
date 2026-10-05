package env

import (
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCatalogConfigSettings(t *testing.T) {
	maxVersions := 3
	cfg := CatalogConfig{
		ID:                   "ide",
		Name:                 map[string]string{"en": "IDE", "fr": "EDI"},
		Status:               StatusProd,
		Highlighted:          []string{"jupyter"},
		Excluded:             []string{"old"},
		AllowSharing:         true,
		Restrictions:         []Restriction{{UserAttributeKey: "groups", Match: "^dev-.*"}},
		MultipleServicesMode: MultipleServicesMaxNumber,
		MaxNumberOfVersions:  &maxVersions,
	}

	s, err := cfg.Settings()

	require.NoError(t, err)
	assert.Equal(t, "ide", s.ID)
	multi, ok := s.Name.GetMulti()
	require.True(t, ok)
	assert.Equal(t, "EDI", multi["fr"])
	assert.Equal(t, domain.CatalogStatusProd, s.Status)
	assert.Equal(t, []string{"jupyter"}, s.Highlighted)
	assert.Equal(t, []string{"old"}, s.Excluded)
	assert.True(t, s.AllowSharing)
	require.Len(t, s.Restrictions, 1)
	assert.Equal(t, "groups", s.Restrictions[0].UserAttributeKey)
	assert.True(t, s.Restrictions[0].Match.MatchString("dev-team"))
	assert.Equal(t, domain.VersionPolicy{Mode: domain.VersionModeMaxNumber, MaxNumber: 3}, s.Versions)
}

func TestCatalogConfigSettings_DefaultsToAllVersions(t *testing.T) {
	s, err := CatalogConfig{ID: "ide", Name: map[string]string{"en": "IDE"}}.Settings()

	require.NoError(t, err)
	assert.Equal(t, domain.VersionModeAll, s.Versions.Mode)
}

func TestCatalogConfigSettings_InvalidRegex(t *testing.T) {
	_, err := CatalogConfig{
		ID:           "ide",
		Name:         map[string]string{"en": "IDE"},
		Restrictions: []Restriction{{UserAttributeKey: "groups", Match: "[invalid"}},
	}.Settings()

	assert.ErrorContains(t, err, "invalid restriction regex")
}

func TestSchemasConfigOverrides_DisabledIgnoresConfig(t *testing.T) {
	o := SchemasConfig{
		Enabled: false,
		Files:   []SchemaFile{{RelativePath: "ide/resources.json", Content: `{"title":"configured"}`}},
	}.Overrides()

	assert.Empty(t, o.Instance)
	assert.Empty(t, o.ByRole)
}

func TestSchemasConfigOverrides_EnabledLoadsConfig(t *testing.T) {
	o := SchemasConfig{
		Enabled: true,
		Files:   []SchemaFile{{RelativePath: "ide/resources.json", Content: `{"title":"configured"}`}},
		Roles: []RoleSchemas{{
			RoleName: "admin",
			Files:    []SchemaFile{{RelativePath: "ide/resources.json", Content: `{"title":"admin-override"}`}},
		}},
	}.Overrides()

	assert.JSONEq(t, `{"title":"configured"}`, string(o.Instance["ide/resources.json"]))
	assert.JSONEq(t, `{"title":"admin-override"}`, string(o.ByRole["admin"]["ide/resources.json"]))
}
