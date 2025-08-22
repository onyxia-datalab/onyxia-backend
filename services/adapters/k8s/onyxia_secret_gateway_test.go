package k8s

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestEnsure_Create(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewSimpleClientset()
	gw := NewK8sOnyxiaSecretGateway(cs)

	ns, name := "user-ddecrulle", "jupyter-python-721817"
	data := map[string][]byte{"owner": []byte("ddecrulle")}

	err := gw.EnsureOnyxiaSecret(ctx, ns, name, data)
	require.NoError(t, err)

	got, err := cs.CoreV1().Secrets(ns).Get(ctx, buildOnyxiaSecretName(name), metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, onyxiaSecretType, got.Type)
	assert.True(t, reflect.DeepEqual(data, got.Data))
}

func TestEnsure_UpdateOnExists(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewSimpleClientset()
	gw := NewK8sOnyxiaSecretGateway(cs)

	ns, name := "ns", "secret"
	// seed
	_, err := cs.CoreV1().Secrets(ns).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: buildOnyxiaSecretName(name), Namespace: ns},
		Type:       corev1.SecretType("other"),
		Data:       map[string][]byte{"owner": []byte("old")},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	newData := map[string][]byte{"owner": []byte("ddecrulle")}
	err = gw.EnsureOnyxiaSecret(ctx, ns, name, newData)
	require.NoError(t, err)

	got, err := cs.CoreV1().Secrets(ns).Get(ctx, buildOnyxiaSecretName(name), metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, onyxiaSecretType, got.Type)
	assert.Equal(t, newData, got.Data)
}

func TestEnsure_RetryOnConflict(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewSimpleClientset()
	gw := NewK8sOnyxiaSecretGateway(cs)

	ns, name := "ns", "secret"
	_, err := cs.CoreV1().Secrets(ns).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: buildOnyxiaSecretName(name), Namespace: ns},
		Type:       onyxiaSecretType,
		Data:       map[string][]byte{"x": []byte("y")},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	conflictedOnce := false
	cs.PrependReactor(
		"update",
		"secrets",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			if !conflictedOnce {
				conflictedOnce = true
				return true, nil, apierrors.NewConflict(
					schema.GroupResource{Resource: "secrets"}, buildOnyxiaSecretName(name), nil)
			}
			return false, nil, nil
		},
	)

	newData := map[string][]byte{"owner": []byte("ddecrulle")}
	err = gw.EnsureOnyxiaSecret(ctx, ns, name, newData)
	require.NoError(t, err)

	got, err := cs.CoreV1().Secrets(ns).Get(ctx, buildOnyxiaSecretName(name), metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, newData, got.Data)
}

func TestEnsure_RecreateIfDeletedDuringUpdate(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewSimpleClientset()
	gw := NewK8sOnyxiaSecretGateway(cs)

	ns, name := "ns", "secret"

	_, err := cs.CoreV1().Secrets(ns).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: buildOnyxiaSecretName(name), Namespace: ns},
		Type:       onyxiaSecretType,
		Data:       map[string][]byte{"x": []byte("y")},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	// First GET will simulate a concurrent deletion by removing the object
	getOnce := false
	cs.PrependReactor(
		"get",
		"secrets",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			if getOnce {
				return false, nil, nil
			}
			getOnce = true

			// Simulate concurrent deletion
			gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
			_ = cs.Tracker().Delete(gvr, ns, buildOnyxiaSecretName(name))

			// Make this first GET look like a NotFound due to concurrent deletion.
			return true, nil, apierrors.NewNotFound(
				schema.GroupResource{Group: "", Resource: "secrets"},
				buildOnyxiaSecretName(name),
			)
		},
	)

	newData := map[string][]byte{"owner": []byte("ddecrulle")}
	err = gw.EnsureOnyxiaSecret(ctx, ns, name, newData)
	require.NoError(t, err)

	got, err := cs.CoreV1().Secrets(ns).Get(ctx, buildOnyxiaSecretName(name), metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, newData, got.Data)
}

func TestDelete_IgnoresNotFound(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewSimpleClientset()
	gw := NewK8sOnyxiaSecretGateway(cs)

	err := gw.DeleteOnyxiaSecret(ctx, "ns", "missing")
	require.NoError(t, err)
}

func TestRead_ReturnsEmptyMapWhenNil(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewSimpleClientset()
	gw := NewK8sOnyxiaSecretGateway(cs)

	ns, name := "ns", "secret-nil"
	_, err := cs.CoreV1().Secrets(ns).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: buildOnyxiaSecretName(name), Namespace: ns},
		Type:       onyxiaSecretType,
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	m, err := gw.ReadOnyxiaSecretData(ctx, ns, name)
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.Empty(t, m)
}
