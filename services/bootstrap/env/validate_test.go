package env

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func validHelmCatalog() CatalogConfig {
	return CatalogConfig{
		Type:     CatalogTypeHelmRepo,
		ID:       "ide",
		Name:     map[string]string{"en": "IDE"},
		Status:   StatusProd,
		Location: "https://charts.example.com",
	}
}

func validOCICatalog() CatalogConfig {
	return CatalogConfig{
		Type:   CatalogTypeOCI,
		ID:     "oci",
		Name:   map[string]string{"en": "OCI"},
		Status: StatusTest,
		Packages: []OCIPackage{
			{Name: "jupyter", ChartRef: "oci://registry.example.com/charts/jupyter"},
		},
	}
}

func intPtr(i int) *int { return &i }

func TestValidateCatalogConfig_Valid(t *testing.T) {
	maxNumber := validHelmCatalog()
	maxNumber.MultipleServicesMode = MultipleServicesMaxNumber
	maxNumber.MaxNumberOfVersions = intPtr(3)

	withTTL := validHelmCatalog()
	withTTL.IndexTTL = time.Minute

	withRestriction := validHelmCatalog()
	withRestriction.Restrictions = []Restriction{{UserAttributeKey: "groups", Match: "^dev-.*"}}

	for name, c := range map[string]CatalogConfig{
		"helm":             validHelmCatalog(),
		"oci":              validOCICatalog(),
		"maxNumber":        maxNumber,
		"helm with ttl":    withTTL,
		"with restriction": withRestriction,
	} {
		t.Run(name, func(t *testing.T) {
			assert.NoError(t, ValidateCatalogConfig(c))
		})
	}
}

func TestValidateCatalogConfig_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CatalogConfig)
		base   func() CatalogConfig
		errMsg string
	}{
		{
			name:   "invalid status",
			base:   validHelmCatalog,
			mutate: func(c *CatalogConfig) { c.Status = "BETA" },
			errMsg: `invalid status "BETA"`,
		},
		{
			name: "maxNumberOfVersions set without maxNumber mode",
			base: validHelmCatalog,
			mutate: func(c *CatalogConfig) {
				c.MultipleServicesMode = MultipleServicesLatest
				c.MaxNumberOfVersions = intPtr(2)
			},
			errMsg: "maxNumberOfVersions must not be set",
		},
		{
			name:   "maxNumber mode without maxNumberOfVersions",
			base:   validHelmCatalog,
			mutate: func(c *CatalogConfig) { c.MultipleServicesMode = MultipleServicesMaxNumber },
			errMsg: "maxNumberOfVersions must be > 0",
		},
		{
			name: "maxNumber mode with non-positive maxNumberOfVersions",
			base: validHelmCatalog,
			mutate: func(c *CatalogConfig) {
				c.MultipleServicesMode = MultipleServicesMaxNumber
				c.MaxNumberOfVersions = intPtr(0)
			},
			errMsg: "maxNumberOfVersions must be > 0",
		},
		{
			name:   "invalid multipleServicesMode",
			base:   validHelmCatalog,
			mutate: func(c *CatalogConfig) { c.MultipleServicesMode = "some" },
			errMsg: `invalid multipleServicesMode "some"`,
		},
		{
			name:   "missing id",
			base:   validHelmCatalog,
			mutate: func(c *CatalogConfig) { c.ID = "" },
			errMsg: "id is required",
		},
		{
			name:   "missing name",
			base:   validHelmCatalog,
			mutate: func(c *CatalogConfig) { c.Name = nil },
			errMsg: "name is required",
		},
		{
			name: "restriction without key",
			base: validHelmCatalog,
			mutate: func(c *CatalogConfig) {
				c.Restrictions = []Restriction{{UserAttributeKey: " ", Match: ".*"}}
			},
			errMsg: "restriction missing userAttribute.key",
		},
		{
			name: "restriction with invalid regex",
			base: validHelmCatalog,
			mutate: func(c *CatalogConfig) {
				c.Restrictions = []Restriction{{UserAttributeKey: "groups", Match: "[invalid"}}
			},
			errMsg: "invalid restriction regex",
		},
		{
			name: "helm catalog with packages",
			base: validHelmCatalog,
			mutate: func(c *CatalogConfig) {
				c.Packages = []OCIPackage{{Name: "jupyter", ChartRef: "oci://x/jupyter"}}
			},
			errMsg: "helm catalog should not have packages",
		},
		{
			name:   "helm catalog with negative ttl",
			base:   validHelmCatalog,
			mutate: func(c *CatalogConfig) { c.IndexTTL = -time.Second },
			errMsg: "indexTtl must not be negative",
		},
		{
			name:   "oci catalog with ttl",
			base:   validOCICatalog,
			mutate: func(c *CatalogConfig) { c.IndexTTL = time.Minute },
			errMsg: "indexTtl is not supported for OCI catalogs",
		},
		{
			name:   "oci catalog with location",
			base:   validOCICatalog,
			mutate: func(c *CatalogConfig) { c.Location = "https://charts.example.com" },
			errMsg: "location must not be set on OCI catalogs",
		},
		{
			name:   "oci catalog without packages",
			base:   validOCICatalog,
			mutate: func(c *CatalogConfig) { c.Packages = nil },
			errMsg: "oci.packages must not be empty",
		},
		{
			name: "oci package without name",
			base: validOCICatalog,
			mutate: func(c *CatalogConfig) {
				c.Packages = []OCIPackage{{ChartRef: "oci://x/jupyter"}}
			},
			errMsg: "oci package: name is required",
		},
		{
			name: "oci package without chartRef",
			base: validOCICatalog,
			mutate: func(c *CatalogConfig) {
				c.Packages = []OCIPackage{{Name: "jupyter"}}
			},
			errMsg: `oci package "jupyter": chartRef is required`,
		},
		{
			name:   "unknown type",
			base:   validHelmCatalog,
			mutate: func(c *CatalogConfig) { c.Type = "git" },
			errMsg: `invalid type "git"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.base()
			tt.mutate(&c)
			assert.ErrorContains(t, ValidateCatalogConfig(c), tt.errMsg)
		})
	}
}

func TestValidateCatalogsConfig(t *testing.T) {
	assert.NoError(t, ValidateCatalogsConfig([]CatalogConfig{validHelmCatalog(), validOCICatalog()}))

	invalid := validHelmCatalog()
	invalid.Status = ""
	assert.ErrorContains(t, ValidateCatalogsConfig([]CatalogConfig{invalid}), "invalid status")

	assert.ErrorContains(
		t,
		ValidateCatalogsConfig([]CatalogConfig{validHelmCatalog(), validHelmCatalog()}),
		`catalog "ide": duplicate id`,
	)
}
