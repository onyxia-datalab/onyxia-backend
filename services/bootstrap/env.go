package bootstrap

import (
	_ "embed"

	"github.com/onyxia-datalab/onyxia-backend/internal/configloader"
)

//go:embed env.default.yaml
var defaultConfig []byte

type Server struct {
	Port        int    `mapstructure:"port"        json:"port"`
	ContextPath string `mapstructure:"contextPath" json:"contextPath"`
}

type OIDC struct {
	IssuerURI     string `mapstructure:"issuerURI"     json:"issuerURI"`
	SkipTLSVerify bool   `mapstructure:"skipTLSVerify" json:"skipTLSVerify"`
	ClientID      string `mapstructure:"clientID"      json:"clientID"`
	Audience      string `mapstructure:"audience"      json:"audience"`
	UsernameClaim string `mapstructure:"usernameClaim" json:"usernameClaim"`
	GroupsClaim   string `mapstructure:"groupsClaim"   json:"groupsClaim"`
	RolesClaim    string `mapstructure:"rolesClaim"    json:"rolesClaim"`
}

type Security struct {
	CORSAllowedOrigins []string `mapstructure:"corsAllowedOrigins" json:"corsAllowedOrigins"`
}

type Env struct {
	AuthenticationMode string   `mapstructure:"authenticationMode" json:"authenticationMode"`
	Server             Server   `mapstructure:"server"             json:"server"`
	OIDC               OIDC     `mapstructure:"oidc"               json:"oidc"`
	Security           Security `mapstructure:"security"           json:"security"`
}

func NewEnv() (Env, error) {
	return configloader.Load[Env](defaultConfig, "env.yaml")
}
