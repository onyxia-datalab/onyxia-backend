package env

// SchemasConfig holds instance-wide and role-specific schema overrides.
// When a chart's values.schema.json contains x-onyxia.overwriteSchemaWith,
// Onyxia resolves the referenced path using this priority chain:
//  1. Role-specific override (first matching role wins)
//  2. Instance-wide override
type SchemasConfig struct {
	// Enabled controls whether admin-configured overrides (Files and Roles) are applied.
	Enabled bool          `mapstructure:"enabled"`
	Files   []SchemaFile  `mapstructure:"files"`
	Roles   []RoleSchemas `mapstructure:"roles"`
}

// SchemaFile maps a relative path (e.g. "ide/resources.json") to raw JSON content.
type SchemaFile struct {
	RelativePath string `mapstructure:"relativePath"`
	Content      string `mapstructure:"content"` // raw JSON
}

// RoleSchemas associates a set of schema overrides to a specific role name.
type RoleSchemas struct {
	RoleName string       `mapstructure:"roleName"`
	Files    []SchemaFile `mapstructure:"files"`
}
