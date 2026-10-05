package env

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/onyxia-datalab/onyxia-backend/internal/tools"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

// CatalogSettings converts the catalogs configuration into the settings the
// catalog use case works with. The configuration must have been validated.
func CatalogSettings(catalogs []CatalogConfig) ([]domain.CatalogSettings, error) {
	out := make([]domain.CatalogSettings, 0, len(catalogs))
	for _, c := range catalogs {
		s, err := c.Settings()
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// Settings converts one catalog's configuration (see CatalogSettings).
func (c CatalogConfig) Settings() (domain.CatalogSettings, error) {
	name, err := tools.NewLocalizedString(c.Name)
	if err != nil {
		return domain.CatalogSettings{}, fmt.Errorf("catalog %q: name: %w", c.ID, err)
	}
	// The description is optional: an absent one stays empty.
	description, _ := tools.NewLocalizedString(c.Description)

	restrictions := make([]domain.CatalogRestriction, 0, len(c.Restrictions))
	for _, r := range c.Restrictions {
		re, err := regexp.Compile(r.Match)
		if err != nil {
			return domain.CatalogSettings{}, fmt.Errorf(
				"catalog %q: invalid restriction regex for key %q: %w", c.ID, r.UserAttributeKey, err,
			)
		}
		restrictions = append(restrictions, domain.CatalogRestriction{
			UserAttributeKey: r.UserAttributeKey,
			Match:            re,
		})
	}

	versions := domain.VersionPolicy{Mode: domain.VersionModeAll}
	if c.MultipleServicesMode != "" {
		versions.Mode = domain.VersionMode(c.MultipleServicesMode)
	}
	if c.MaxNumberOfVersions != nil {
		versions.MaxNumber = *c.MaxNumberOfVersions
	}

	return domain.CatalogSettings{
		ID:           c.ID,
		Name:         name,
		Description:  description,
		Status:       domain.CatalogStatus(c.Status),
		Highlighted:  append([]string(nil), c.Highlighted...),
		Excluded:     append([]string(nil), c.Excluded...),
		AllowSharing: c.AllowSharing,
		Restrictions: restrictions,
		Versions:     versions,
	}, nil
}

// Overrides converts the schema overrides configuration. Disabled overrides
// convert to empty ones.
func (s SchemasConfig) Overrides() domain.SchemaOverrides {
	out := domain.SchemaOverrides{
		Instance: map[string]json.RawMessage{},
		ByRole:   map[string]map[string]json.RawMessage{},
	}
	if !s.Enabled {
		return out
	}
	for _, f := range s.Files {
		out.Instance[f.RelativePath] = json.RawMessage(f.Content)
	}
	for _, role := range s.Roles {
		files := make(map[string]json.RawMessage, len(role.Files))
		for _, f := range role.Files {
			files[f.RelativePath] = json.RawMessage(f.Content)
		}
		out.ByRole[role.RoleName] = files
	}
	return out
}
