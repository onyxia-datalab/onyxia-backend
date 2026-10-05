package domain

import (
	"encoding/json"
	"regexp"

	"github.com/onyxia-datalab/onyxia-backend/internal/tools"
)

// CatalogSettings is how the instance configures a catalog: who may use it,
// which of its packages and versions are exposed, and whether the services
// installed from it may be shared. Where the charts come from (repository,
// credentials) is the package repository's concern, not this one's.
type CatalogSettings struct {
	ID           string
	Name         tools.LocalizedString
	Description  tools.LocalizedString
	Status       CatalogStatus
	Highlighted  []string
	Excluded     []string
	AllowSharing bool
	// Restrictions make the catalog visible only to users matching at least
	// one of them. A catalog without restrictions is public.
	Restrictions []CatalogRestriction
	Versions     VersionPolicy
}

// CatalogRestriction matches a user attribute (a token claim) against a
// regular expression.
type CatalogRestriction struct {
	UserAttributeKey string
	Match            *regexp.Regexp
}

// VersionPolicy selects which of a package's versions are offered.
type VersionPolicy struct {
	Mode VersionMode
	// MaxNumber is the number of versions kept with VersionModeMaxNumber.
	MaxNumber int
}

type VersionMode string

const (
	VersionModeAll         VersionMode = "all"
	VersionModeLatest      VersionMode = "latest"
	VersionModeSkipPatches VersionMode = "skipPatches"
	VersionModeMaxNumber   VersionMode = "maxNumber"
)

// SchemaOverrides are the schemas that replace the nodes of a package's
// values schema carrying x-onyxia.overwriteSchemaWith, keyed by the
// referenced relative path. A role-specific override wins over an
// instance-wide one.
type SchemaOverrides struct {
	Instance map[string]json.RawMessage
	ByRole   map[string]map[string]json.RawMessage // role name -> relative path -> schema
}
