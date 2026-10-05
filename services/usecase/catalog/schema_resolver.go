package catalog

import (
	"encoding/json"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

// schemaResolver resolves overwriteSchemaWith references in priority order:
//  1. Role-specific override (first matching role wins)
//  2. Instance-wide override
type schemaResolver struct {
	instanceFiles map[string]json.RawMessage
	roleFiles     map[string]map[string]json.RawMessage // roleName -> relativePath -> content
}

func newSchemaResolver(overrides domain.SchemaOverrides) *schemaResolver {
	r := &schemaResolver{
		instanceFiles: overrides.Instance,
		roleFiles:     overrides.ByRole,
	}
	if r.instanceFiles == nil {
		r.instanceFiles = map[string]json.RawMessage{}
	}
	if r.roleFiles == nil {
		r.roleFiles = map[string]map[string]json.RawMessage{}
	}
	return r
}

// resolve looks up a schema by relative path for the given user roles.
// Returns the resolved JSON and true on success, or nil and false if not found.
func (r *schemaResolver) resolve(path string, userRoles []string) (json.RawMessage, bool) {
	// 1. Role-specific overrides (first matching role wins)
	for _, role := range userRoles {
		if files, ok := r.roleFiles[role]; ok {
			if content, ok := files[path]; ok {
				return content, true
			}
		}
	}
	// 2. Instance-wide overrides
	if content, ok := r.instanceFiles[path]; ok {
		return content, true
	}
	return nil, false
}

// applyOverwrites walks the schema JSON tree and replaces any node containing
// x-onyxia.overwriteSchemaWith with the resolved schema content.
// If the schema is not valid JSON or a node cannot be resolved, it is left unchanged.
func applyOverwrites(schema []byte, r *schemaResolver, userRoles []string) ([]byte, error) {
	var root any
	if err := json.Unmarshal(schema, &root); err != nil {
		// Not valid JSON — return as-is rather than failing the request.
		return schema, nil
	}
	result := walkNode(root, r, userRoles)
	return json.Marshal(result)
}

func walkNode(node any, r *schemaResolver, userRoles []string) any {
	switch v := node.(type) {
	case map[string]any:
		// If this node declares overwriteSchemaWith, replace the whole node.
		if xOnyxia, ok := v["x-onyxia"].(map[string]any); ok {
			if path, ok := xOnyxia["overwriteSchemaWith"].(string); ok && path != "" {
				if resolved, ok := r.resolve(path, userRoles); ok {
					var replacement any
					if err := json.Unmarshal(resolved, &replacement); err == nil {
						return replacement
					}
				}
			}
		}
		// Recurse into all child nodes.
		for k, val := range v {
			v[k] = walkNode(val, r, userRoles)
		}
		return v
	case []any:
		for i, elem := range v {
			v[i] = walkNode(elem, r, userRoles)
		}
		return v
	default:
		return node
	}
}
