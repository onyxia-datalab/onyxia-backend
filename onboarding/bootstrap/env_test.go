package bootstrap

import (
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/internal/configloader"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultEnvIsValid(t *testing.T) {
	env, err := NewEnv()

	require.NoError(t, err)
	assert.Empty(t, env.Onboarding.NamespaceResources)
}

// Manifests are block strings, so Viper keeps the case of their keys.
func TestResourcesFromYAMLKeepKeyCase(t *testing.T) {
	yaml := []byte(`
onboarding:
  namespaceResources:
    - |
      apiVersion: v1
      kind: ServiceAccount
      metadata:
        name: onyxia-installer-sa
      automountServiceAccountToken: false
    - |
      apiVersion: rbac.authorization.k8s.io/v1
      kind: RoleBinding
      metadata:
        name: onyxia-installer-rb
      roleRef:
        apiGroup: rbac.authorization.k8s.io
        kind: Role
        name: onyxia-installer-role
      subjects:
        - kind: ServiceAccount
          name: onyxia-installer-sa
          namespace: ${onyxia.namespace}
`)

	env, err := configloader.Load[Env](yaml, "")
	require.NoError(t, err)

	resources, err := ParseResources(env.Onboarding.NamespaceResources)

	require.NoError(t, err)
	require.Len(t, resources, 2)
	assert.Equal(t, domain.Resource{
		"apiVersion":                   "v1",
		"kind":                         "ServiceAccount",
		"metadata":                     map[string]any{"name": "onyxia-installer-sa"},
		"automountServiceAccountToken": false,
	}, resources[0])
	assert.Equal(t, "${onyxia.namespace}",
		resources[1]["subjects"].([]any)[0].(map[string]any)["namespace"])
}

func TestParseResourcesKeepsIntegers(t *testing.T) {
	resources, err := ParseResources([]string{`
apiVersion: v1
kind: ResourceQuota
metadata:
  name: extra
spec:
  hard:
    count/pods: 5
`})

	require.NoError(t, err)
	assert.Equal(t, int64(5), resources[0]["spec"].(map[string]any)["hard"].(map[string]any)["count/pods"])
}

func TestParseResourcesAcceptsShellVariables(t *testing.T) {
	_, err := ParseResources([]string{`
apiVersion: v1
kind: ConfigMap
metadata:
  name: init-script
data:
  run.sh: |
    cd "${HOME}" && echo "${onyxia.namespace}"
`})

	assert.NoError(t, err)
}

func TestParseResourcesRejectsInvalidManifests(t *testing.T) {
	tests := map[string]string{
		"not yaml":            "kind: [",
		"no kind":             "apiVersion: v1\nmetadata:\n  name: a\n",
		"no apiVersion":       "kind: ConfigMap\nmetadata:\n  name: a\n",
		"no name":             "apiVersion: v1\nkind: ConfigMap\n",
		"namespace set":       "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n  namespace: kube-system\n",
		"several objects":     "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: b\n",
		"unknown placeholder": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: ${onyxia.user}\n",
	}

	for name, manifest := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseResources([]string{manifest})
			assert.Error(t, err)
		})
	}
}
