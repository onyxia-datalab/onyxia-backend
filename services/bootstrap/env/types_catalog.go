package env

type CatalogConfig struct {
	Type CatalogType `json:"type"` // "helm" or "oci"`

	// Common fields
	ID            string            `mapstructure:"id"                json:"id"`
	Name          map[string]string `mapstructure:"name"              json:"name"`
	Description   map[string]string `mapstructure:"description"       json:"description"`
	Maintainer    string            `mapstructure:"maintainer"        json:"maintainer"`
	Status        CatalogStatus     `mapstructure:"status"            json:"status"`
	Highlighted   []string          `mapstructure:"highlightedCharts" json:"highlightedCharts"`
	Excluded      []string          `mapstructure:"excludedCharts"    json:"excludedCharts"`
	SkipTLSVerify bool              `mapstructure:"skipTlsVerify"     json:"skipTlsVerify"`
	CAFile        *string           `mapstructure:"caFile"            json:"caFile"`
	AllowSharing  bool              `mapstructure:"allowSharing"      json:"allowSharing"`
	Restrictions  []Restriction     `mapstructure:"restrictions"      json:"restrictions"`
	Username      *string           `mapstructure:"username"          json:"username"`
	Password      *string           `mapstructure:"password"          json:"password"`
	Location      string            `mapstructure:"location"          json:"location"`

	MultipleServicesMode MultipleServicesMode `mapstructure:"multipleServicesMode" json:"multipleServicesMode"`
	MaxNumberOfVersions  *int                 `mapstructure:"maxNumberOfVersions"  json:"maxNumberOfVersions,omitempty"`

	// Specific to OCI
	Packages []OCIPackage `json:"packages,omitempty"`
}

//Enums

type CatalogType string

const (
	CatalogTypeHelm CatalogType = "helm"
	CatalogTypeOCI  CatalogType = "oci"
)

type CatalogStatus string

const (
	StatusProd CatalogStatus = "PROD"
	StatusTest CatalogStatus = "TEST"
)

type MultipleServicesMode string

const (
	MultipleServicesAll         MultipleServicesMode = "all"
	MultipleServicesLatest      MultipleServicesMode = "latest"
	MultipleServicesSkipPatches MultipleServicesMode = "skipPatches"
	MultipleServicesMaxNumber   MultipleServicesMode = "maxNumber"
)

type Restriction struct {
	UserAttributeKey string `mapstructure:"userAttribute.key"     json:"userAttributeKey"`
	Match            string `mapstructure:"userAttribute.matches" json:"match"`
}

type OCIPackage struct {
	Name     string   `json:"name"`
	Versions []string `json:"versions"`
}
