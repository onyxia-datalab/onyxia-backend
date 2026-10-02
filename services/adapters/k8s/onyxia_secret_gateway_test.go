package k8s

import (
	"context"
	"reflect"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
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

func TestCreate(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewClientset()
	gw := NewOnyxiaSecretGtw(cs)

	ns, name := "user-ddecrulle", "jupyter-python-721817"
	data := map[string][]byte{"owner": []byte("ddecrulle")}

	err := gw.CreateOnyxiaSecret(ctx, ns, name, data)
	require.NoError(t, err)

	got, err := cs.CoreV1().Secrets(ns).Get(ctx, buildOnyxiaSecretName(name), metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, onyxiaSecretType, got.Type)
	assert.True(t, reflect.DeepEqual(data, got.Data))
}

func TestCreateReturnsAlreadyExists(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewClientset()
	gw := NewOnyxiaSecretGtw(cs)

	ns, name := "ns", "secret"
	// seed
	_, err := cs.CoreV1().Secrets(ns).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: buildOnyxiaSecretName(name), Namespace: ns},
		Type:       corev1.SecretType("other"),
		Data:       map[string][]byte{"owner": []byte("old")},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	err = gw.CreateOnyxiaSecret(ctx, ns, name, map[string][]byte{"owner": []byte("ddecrulle")})
	require.ErrorIs(t, err, domain.ErrAlreadyExists)

	got, err := cs.CoreV1().Secrets(ns).Get(ctx, buildOnyxiaSecretName(name), metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, corev1.SecretType("other"), got.Type)
	assert.Equal(t, map[string][]byte{"owner": []byte("old")}, got.Data)
}

func TestUpdateRetryOnConflict(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewClientset()
	gw := NewOnyxiaSecretGtw(cs)

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
	err = gw.UpdateOnyxiaSecret(ctx, ns, name, newData)
	require.NoError(t, err)

	got, err := cs.CoreV1().Secrets(ns).Get(ctx, buildOnyxiaSecretName(name), metav1.GetOptions{})
	require.NoError(t, err)
	// Update merges: keys not passed in are kept.
	assert.Equal(t, map[string][]byte{"x": []byte("y"), "owner": []byte("ddecrulle")}, got.Data)
}

func TestUpdateReturnsNotFoundIfDeletedDuringUpdate(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewClientset()
	gw := NewOnyxiaSecretGtw(cs)

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
	err = gw.UpdateOnyxiaSecret(ctx, ns, name, newData)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDeleteIgnoresNotFound(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewClientset()
	gw := NewOnyxiaSecretGtw(cs)

	err := gw.DeleteOnyxiaSecret(ctx, "ns", "missing")
	require.NoError(t, err)
}

func TestReadReturnsEmptyMapWhenNil(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewClientset()
	gw := NewOnyxiaSecretGtw(cs)

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
