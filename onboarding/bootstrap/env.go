package bootstrap

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/onyxia-datalab/onyxia-backend/internal/configloader"
	"github.com/onyxia-datalab/onyxia-backend/internal/server"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
)

//go:embed env.default.yaml
var defaultConfig []byte

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

type Quota struct {
	RequestsMemory           string `mapstructure:"requests.memory"            json:"requests.memory"`
	RequestsCPU              string `mapstructure:"requests.cpu"               json:"requests.cpu"`
	LimitsMemory             string `mapstructure:"limits.memory"              json:"limits.memory"`
	LimitsCPU                string `mapstructure:"limits.cpu"                 json:"limits.cpu"`
	RequestsStorage          string `mapstructure:"requests.storage"           json:"requests.storage"`
	CountPods                string `mapstructure:"count/pods"                 json:"count/pods"`
	RequestsEphemeralStorage string `mapstructure:"requests.ephemeral-storage" json:"requests.ephemeral-storage"`
	LimitsEphemeralStorage   string `mapstructure:"limits.ephemeral-storage"   json:"limits.ephemeral-storage"`
	RequestsGPU              string `mapstructure:"requests.nvidia.com/gpu"    json:"requests.nvidia.com/gpu"`
	LimitsGPU                string `mapstructure:"limits.nvidia.com/gpu"      json:"limits.nvidia.com/gpu"`
}

type Quotas struct {
	Enabled      bool             `mapstructure:"enabled"      json:"enabled"`
	Default      Quota            `mapstructure:"default"      json:"default"`
	UserEnabled  bool             `mapstructure:"userEnabled"  json:"userEnabled"`
	User         Quota            `mapstructure:"user"         json:"user"`
	GroupEnabled bool             `mapstructure:"groupEnabled" json:"groupEnabled"`
	Group        Quota            `mapstructure:"group"        json:"group"`
	Roles        map[string]Quota `mapstructure:"roles"        json:"roles"`
}

type Annotation struct {
	Enabled bool              `mapstructure:"enabled" json:"enabled"`
	Static  map[string]string `mapstructure:"static"  json:"static"`
	Dynamic struct {
		LastLoginTimestamp bool     `mapstructure:"last-login-timestamp" json:"last-login-timestamp"`
		UserAttributes     []string `mapstructure:"userAttributes" json:"userAttributes"`
	} `mapstructure:"dynamic" json:"dynamic"`
}
type Onboarding struct {
	NamespacePrefix      string            `mapstructure:"namespacePrefix"      json:"namespacePrefix"`
	NamespaceLabels      map[string]string `mapstructure:"namespaceLabels"      json:"labels"`
	GroupNamespacePrefix string            `mapstructure:"groupNamespacePrefix" json:"groupNamespacePrefix"`
	Annotation           Annotation        `mapstructure:"annotations"          json:"annotations"`
	Quotas               Quotas            `mapstructure:"quotas"               json:"quotas"`
	// NamespaceResources are YAML manifests, one object per entry. They are kept as
	// strings because Viper lowercases every key it loads, which would
	// break fields such as apiVersion.
	NamespaceResources []string `mapstructure:"namespaceResources" json:"namespaceResources"`
}

type Env struct {
	AuthenticationMode string        `mapstructure:"authenticationMode" json:"authenticationMode"`
	Server             server.Config `mapstructure:"server"             json:"server"`
	OIDC               OIDC          `mapstructure:"oidc"               json:"oidc"`
	Security           Security      `mapstructure:"security"           json:"security"`
	Onboarding         Onboarding    `mapstructure:"onboarding"         json:"onboarding"`
}

func NewEnv() (Env, error) {
	env, err := configloader.Load[Env](defaultConfig, "env.onboarding.yaml")
	if err != nil {
		return env, err
	}
	if _, err := ParseResources(env.Onboarding.NamespaceResources); err != nil {
		return env, err
	}
	return env, nil
}

// ParseResources parses the configured manifests and rejects, at startup
// rather than at the first onboarding, those that can never be applied.
// Whether their kind exists and is namespaced is checked against the
// cluster by the adapter.
func ParseResources(manifests []string) ([]domain.Resource, error) {
	resources := make([]domain.Resource, 0, len(manifests))
	for i, manifest := range manifests {
		resource, err := parseResource(manifest)
		if err != nil {
			return nil, fmt.Errorf("invalid onboarding.namespaceResources[%d]: %w", i, err)
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func parseResource(manifest string) (domain.Resource, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest), 4096)

	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("a manifest must contain exactly one object")
	}

	// Unlike a plain json.Unmarshal, keeps integers as int64.
	obj := &unstructured.Unstructured{}
	if err := obj.UnmarshalJSON(raw); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	switch {
	case obj.GetAPIVersion() == "":
		return nil, errors.New("apiVersion is required")
	case obj.GetName() == "":
		return nil, errors.New("metadata.name is required")
	case obj.GetNamespace() != "":
		return nil, errors.New("metadata.namespace must not be set: resources are always applied in the onboarded namespace")
	}

	resource := domain.Resource(obj.Object)
	if unknown := resource.UnknownPlaceholders(); len(unknown) > 0 {
		return nil, fmt.Errorf("unknown placeholders %v: use ${onyxia.%s}, ${onyxia.%s} or ${onyxia.%s}", unknown,
			domain.PlaceholderNamespace, domain.PlaceholderUsername, domain.PlaceholderGroup)
	}
	return resource, nil
}
