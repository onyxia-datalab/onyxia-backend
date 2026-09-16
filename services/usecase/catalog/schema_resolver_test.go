package catalog

import (
	"encoding/json"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------- helpers ----------

func resolverWith(instanceFiles map[string]string) *schemaResolver {
	r := &schemaResolver{
		instanceFiles: make(map[string]json.RawMessage),
		roleFiles:     make(map[string]map[string]json.RawMessage),
	}
	for path, content := range instanceFiles {
		r.instanceFiles[path] = json.RawMessage(content)
	}
	return r
}

func unmarshal(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}

// ---------- resolve ----------

// ✅ Instance override is found.
func TestResolve_InstanceFile(t *testing.T) {
	r := resolverWith(map[string]string{
		"ide/resources.json": `{"title":"instance"}`,
	})

	content, ok := r.resolve("ide/resources.json", nil)

	assert.True(t, ok)
	got := unmarshal(t, content)
	assert.Equal(t, "instance", got["title"])
}

// ✅ Role override takes precedence over instance override.
func TestResolve_RoleOverInstance(t *testing.T) {
	r := resolverWith(map[string]string{
		"ide/resources.json": `{"title":"instance"}`,
	})
	r.roleFiles["fullgpu"] = map[string]json.RawMessage{
		"ide/resources.json": json.RawMessage(`{"title":"role"}`),
	}

	content, ok := r.resolve("ide/resources.json", []string{"fullgpu"})

	assert.True(t, ok)
	got := unmarshal(t, content)
	assert.Equal(t, "role", got["title"])
}

// ✅ First matching role wins when user has multiple roles.
func TestResolve_FirstMatchingRoleWins(t *testing.T) {
	r := resolverWith(map[string]string{})
	r.roleFiles["roleA"] = map[string]json.RawMessage{
		"ide/resources.json": json.RawMessage(`{"title":"roleA"}`),
	}
	r.roleFiles["roleB"] = map[string]json.RawMessage{
		"ide/resources.json": json.RawMessage(`{"title":"roleB"}`),
	}

	content, ok := r.resolve("ide/resources.json", []string{"roleA", "roleB"})

	assert.True(t, ok)
	got := unmarshal(t, content)
	assert.Equal(t, "roleA", got["title"])
}

// ✅ User role with no matching file falls through to instance override.
func TestResolve_RoleMissingPathFallsThrough(t *testing.T) {
	r := resolverWith(map[string]string{
		"ide/resources.json": `{"title":"instance"}`,
	})
	r.roleFiles["fullgpu"] = map[string]json.RawMessage{} // role exists but not for this path

	content, ok := r.resolve("ide/resources.json", []string{"fullgpu"})

	assert.True(t, ok)
	got := unmarshal(t, content)
	assert.Equal(t, "instance", got["title"])
}

// ❌ Unknown path returns false.
func TestResolve_UnknownPath(t *testing.T) {
	r := resolverWith(map[string]string{})

	_, ok := r.resolve("unknown/path.json", nil)

	assert.False(t, ok)
}

// ---------- walkNode ----------

// ✅ Node with overwriteSchemaWith is replaced with the resolved schema.
func TestWalkNode_ReplacesNode(t *testing.T) {
	r := resolverWith(map[string]string{
		"ide/customImage.json": `{"type":"boolean","default":false}`,
	})
	input := map[string]any{
		"x-onyxia": map[string]any{
			"overwriteSchemaWith": "ide/customImage.json",
		},
		"type": "string", // original type — should be gone after replacement
	}

	result := walkNode(input, r, nil)

	got, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "boolean", got["type"])
	_, hasXOnyxia := got["x-onyxia"]
	assert.False(t, hasXOnyxia)
}

// ✅ Nested nodes are walked recursively.
func TestWalkNode_Recurses(t *testing.T) {
	r := resolverWith(map[string]string{
		"ide/resources.json": `{"title":"resources"}`,
	})
	input := map[string]any{
		"properties": map[string]any{
			"resources": map[string]any{
				"x-onyxia": map[string]any{
					"overwriteSchemaWith": "ide/resources.json",
				},
			},
		},
	}

	result := walkNode(input, r, nil)

	got := result.(map[string]any)
	props := got["properties"].(map[string]any)
	resources := props["resources"].(map[string]any)
	assert.Equal(t, "resources", resources["title"])
}

// ✅ Arrays are walked element by element.
func TestWalkNode_WalksArrays(t *testing.T) {
	r := resolverWith(map[string]string{
		"ide/customImage.json": `{"type":"boolean"}`,
	})
	input := []any{
		map[string]any{
			"x-onyxia": map[string]any{"overwriteSchemaWith": "ide/customImage.json"},
		},
		map[string]any{"type": "string"},
	}

	result := walkNode(input, r, nil)

	arr := result.([]any)
	first := arr[0].(map[string]any)
	assert.Equal(t, "boolean", first["type"])
	second := arr[1].(map[string]any)
	assert.Equal(t, "string", second["type"])
}

// ✅ Unknown path leaves node unchanged.
func TestWalkNode_UnknownPathUnchanged(t *testing.T) {
	r := resolverWith(map[string]string{})
	input := map[string]any{
		"x-onyxia": map[string]any{"overwriteSchemaWith": "unknown.json"},
		"type":     "object",
	}

	result := walkNode(input, r, nil)

	got := result.(map[string]any)
	assert.Equal(t, "object", got["type"])
	assert.NotNil(t, got["x-onyxia"])
}

// ✅ Non-object, non-array nodes pass through untouched.
func TestWalkNode_ScalarPassthrough(t *testing.T) {
	r := resolverWith(map[string]string{})

	assert.Equal(t, "hello", walkNode("hello", r, nil))
	assert.Equal(t, 42.0, walkNode(42.0, r, nil))
	assert.Equal(t, true, walkNode(true, r, nil))
	assert.Nil(t, walkNode(nil, r, nil))
}

// ---------- applyOverwrites ----------

// ✅ applyOverwrites substitutes overwriteSchemaWith in a full schema document.
func TestApplyOverwrites_FullDocument(t *testing.T) {
	r := resolverWith(map[string]string{
		"ide/git.json": `{"title":"git","type":"object"}`,
	})
	schema := []byte(`{
		"properties": {
			"git": {
				"x-onyxia": {"overwriteSchemaWith": "ide/git.json"}
			}
		}
	}`)

	result, err := applyOverwrites(schema, r, nil)

	require.NoError(t, err)
	got := unmarshal(t, result)
	props := got["properties"].(map[string]any)
	git := props["git"].(map[string]any)
	assert.Equal(t, "git", git["title"])
}

// ✅ applyOverwrites returns invalid JSON as-is without error.
func TestApplyOverwrites_InvalidJSONPassthrough(t *testing.T) {
	r := resolverWith(map[string]string{})
	invalid := []byte(`not json at all`)

	result, err := applyOverwrites(invalid, r, nil)

	assert.NoError(t, err)
	assert.Equal(t, invalid, result)
}

// ---------- newSchemaResolver ----------

// ✅ When Enabled=false, configured files and roles are ignored.
func TestNewSchemaResolver_DisabledIgnoresConfig(t *testing.T) {
	cfg := env.SchemasConfig{
		Enabled: false,
		Files: []env.SchemaFile{
			{RelativePath: "ide/resources.json", Content: `{"title":"configured"}`},
		},
	}
	r := newSchemaResolver(cfg)
	_, ok := r.instanceFiles["ide/resources.json"]
	assert.False(t, ok)
}

// ✅ When Enabled=true, configured files and roles are loaded.
func TestNewSchemaResolver_EnabledLoadsConfig(t *testing.T) {
	cfg := env.SchemasConfig{
		Enabled: true,
		Files: []env.SchemaFile{
			{RelativePath: "ide/resources.json", Content: `{"title":"configured"}`},
		},
		Roles: []env.RoleSchemas{
			{
				RoleName: "admin",
				Files: []env.SchemaFile{
					{RelativePath: "ide/resources.json", Content: `{"title":"admin-override"}`},
				},
			},
		},
	}
	r := newSchemaResolver(cfg)

	assert.Contains(t, r.instanceFiles, "ide/resources.json")
	assert.Contains(t, r.roleFiles, "admin")
	assert.Contains(t, r.roleFiles["admin"], "ide/resources.json")
}
