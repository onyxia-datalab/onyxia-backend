package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

func testMapper() meta.RESTMapper {
	mapper := meta.NewDefaultRESTMapper(nil)
	mapper.Add(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "Role"}, meta.RESTScopeNamespace)
	mapper.Add(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}, meta.RESTScopeRoot)
	return mapper
}

func testResource(kind string) domain.Resource {
	return domain.Resource{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       kind,
		"metadata":   map[string]any{"name": "onyxia-installer-role"},
		"rules": []any{map[string]any{
			"apiGroups": []any{""},
			"resources": []any{"pods"},
			"verbs":     []any{"get"},
		}},
	}
}

func TestCheckResource(t *testing.T) {
	applier := NewKubernetesResourceApplier(dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()), testMapper())

	assert.NoError(t, applier.CheckResource(testResource("Role")))
	assert.ErrorContains(t, applier.CheckResource(testResource("ClusterRole")), "cluster-scoped")
	assert.ErrorContains(t, applier.CheckResource(testResource("Unknown")), "unknown kind")
}

// applyRecorder captures the server-side apply requests, which the fake
// dynamic client cannot execute.
func applyRecorder(client *dynamicfake.FakeDynamicClient) *[]k8stesting.PatchActionImpl {
	var actions []k8stesting.PatchActionImpl
	client.PrependReactor("patch", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
		actions = append(actions, action.(k8stesting.PatchActionImpl))
		return true, nil, nil
	})
	return &actions
}

func TestApplyResource(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	actions := applyRecorder(client)
	applier := NewKubernetesResourceApplier(client, testMapper())

	err := applier.ApplyResource(context.Background(), "user-test", testResource("Role"))

	require.NoError(t, err)
	require.Len(t, *actions, 1)
	action := (*actions)[0]
	assert.Equal(t, "user-test", action.GetNamespace())
	assert.Equal(t, "onyxia-installer-role", action.GetName())
	assert.Equal(t, schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}, action.GetResource())
	assert.Equal(t, types.ApplyPatchType, action.GetPatchType())
	assert.Equal(t, metav1.PatchOptions{FieldManager: FieldManager, Force: new(true)}, action.PatchOptions)

	var applied map[string]any
	require.NoError(t, json.Unmarshal(action.GetPatch(), &applied))
	metadata := applied["metadata"].(map[string]any)
	assert.Equal(t, "user-test", metadata["namespace"])
	assert.Equal(t, map[string]any{"created-by": "onyxia"}, metadata["labels"])
	assert.NotEmpty(t, applied["rules"])
}

func TestApplyResourceKeepsConfiguredLabels(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	actions := applyRecorder(client)
	applier := NewKubernetesResourceApplier(client, testMapper())
	resource := testResource("Role")
	resource["metadata"].(map[string]any)["labels"] = map[string]any{"created-by": "admin", "team": "data"}

	require.NoError(t, applier.ApplyResource(context.Background(), "user-test", resource))

	var applied map[string]any
	require.NoError(t, json.Unmarshal((*actions)[0].GetPatch(), &applied))
	assert.Equal(t, map[string]any{"created-by": "admin", "team": "data"},
		applied["metadata"].(map[string]any)["labels"])
	// The configured resource is shared by every onboarding: it must not change.
	_, hasNamespace := resource["metadata"].(map[string]any)["namespace"]
	assert.False(t, hasNamespace)
}

func TestApplyResourceRefusesClusterScopedKind(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	actions := applyRecorder(client)
	applier := NewKubernetesResourceApplier(client, testMapper())

	err := applier.ApplyResource(context.Background(), "user-test", testResource("ClusterRole"))

	assert.ErrorContains(t, err, "cluster-scoped")
	assert.Empty(t, *actions)
}

func TestApplyResourceError(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	client.PrependReactor("patch", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("forbidden")
	})
	applier := NewKubernetesResourceApplier(client, testMapper())

	err := applier.ApplyResource(context.Background(), "user-test", testResource("Role"))

	assert.ErrorContains(t, err, "forbidden")
}
