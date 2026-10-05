package env

import "github.com/onyxia-datalab/onyxia-backend/internal/server"

type OIDC struct {
	IssuerURI     string `mapstructure:"issuerURI"     json:"issuerURI"`
	SkipTLSVerify bool   `mapstructure:"skipTLSVerify" json:"skipTLSVerify"`
	PublicKey     string `mapstructure:"publicKey"     json:"publicKey"`
	Audience      string `mapstructure:"audience"      json:"audience"`
	UsernameClaim string `mapstructure:"usernameClaim" json:"usernameClaim"`
	GroupsClaim   string `mapstructure:"groupsClaim"   json:"groupsClaim"`
	RolesClaim    string `mapstructure:"rolesClaim"    json:"rolesClaim"`
}

type Security struct {
	CORSAllowedOrigins []string `mapstructure:"corsAllowedOrigins" json:"corsAllowedOrigins"`
}

type Kubernetes struct {
	NamespacePrefix      string `mapstructure:"namespacePrefix"      json:"namespacePrefix"`
	GroupNamespacePrefix string `mapstructure:"groupNamespacePrefix" json:"groupNamespacePrefix"`
}
type Env struct {
	AuthenticationMode string          `mapstructure:"authenticationMode" json:"authenticationMode"`
	Server             server.Config   `mapstructure:"server"             json:"server"`
	OIDC               OIDC            `mapstructure:"oidc"               json:"oidc"`
	Security           Security        `mapstructure:"security"           json:"security"`
	CatalogsConfig     []CatalogConfig `mapstructure:"catalogs"           json:"catalogs"`
	Kubernetes         Kubernetes      `mapstructure:"kubernetes"         json:"kubernetes"`
	Schemas            SchemasConfig   `mapstructure:"schemas"            json:"schemas"`
}
