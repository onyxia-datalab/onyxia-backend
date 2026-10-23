package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderReplacesPlaceholdersInStringValues(t *testing.T) {
	resource := Resource{
		"kind": "RoleBinding",
		"metadata": map[string]any{
			"name":   "${onyxia.username}-rb",
			"labels": map[string]any{"group": "${onyxia.group}"},
		},
		"subjects": []any{
			map[string]any{"kind": "ServiceAccount", "namespace": "${onyxia.namespace}"},
		},
		"replicas": int64(2),
		"enabled":  true,
	}

	rendered := resource.Render(map[string]string{
		PlaceholderNamespace: "projet-test",
		PlaceholderUsername:  "jdoe",
		PlaceholderGroup:     "test",
	})

	assert.Equal(t, Resource{
		"kind": "RoleBinding",
		"metadata": map[string]any{
			"name":   "jdoe-rb",
			"labels": map[string]any{"group": "test"},
		},
		"subjects": []any{
			map[string]any{"kind": "ServiceAccount", "namespace": "projet-test"},
		},
		"replicas": int64(2),
		"enabled":  true,
	}, rendered)
	// The configured resource is shared by every onboarding: it must not change.
	assert.Equal(t, "${onyxia.username}-rb", resource["metadata"].(map[string]any)["name"])
}

func TestRenderMissingValueIsEmpty(t *testing.T) {
	rendered := Resource{"group": "g-${onyxia.group}"}.Render(map[string]string{PlaceholderNamespace: "user-jdoe"})

	assert.Equal(t, Resource{"group": "g-"}, rendered)
}

// A value coming from the user's token stays a plain string: it cannot add
// fields to the object, whatever it contains.
func TestRenderValueCannotInjectFields(t *testing.T) {
	rendered := Resource{"metadata": map[string]any{"name": "${onyxia.username}"}}.Render(map[string]string{
		PlaceholderUsername: "jdoe\nkind: ClusterRoleBinding",
	})

	assert.Equal(t, Resource{"metadata": map[string]any{"name": "jdoe\nkind: ClusterRoleBinding"}}, rendered)
}

func TestUnknownPlaceholders(t *testing.T) {
	resource := Resource{
		"a": "${onyxia.namespace}-${onyxia.user}",
		"b": []any{"${onyxia.nmespace}", "${onyxia.user}", "$notAPlaceholder"},
	}

	assert.ElementsMatch(t, []string{"${onyxia.user}", "${onyxia.nmespace}"}, resource.UnknownPlaceholders())
	assert.Empty(t, Resource{"a": "${onyxia.namespace} ${onyxia.username} ${onyxia.group}"}.UnknownPlaceholders())
}

// Other ${...}, such as shell variables in a script, are neither replaced
// nor reported.
func TestNonOnyxiaPlaceholdersAreLeftAlone(t *testing.T) {
	resource := Resource{"data": map[string]any{
		"run.sh": "echo ${HOME} ${namespace} in ${onyxia.namespace}",
	}}

	assert.Empty(t, resource.UnknownPlaceholders())
	assert.Equal(t, Resource{"data": map[string]any{
		"run.sh": "echo ${HOME} ${namespace} in user-jdoe",
	}}, resource.Render(map[string]string{PlaceholderNamespace: "user-jdoe"}))
}
