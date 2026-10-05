package k8s

import (
	"context"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
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

func seedSecret(t *testing.T, cs *k8sfake.Clientset, ns, name string, typ corev1.SecretType, data map[string][]byte) {
	t.Helper()
	_, err := cs.CoreV1().Secrets(ns).Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Type:       typ,
		Data:       data,
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}

// The secret layout is shared with onyxia-api (Java): both APIs must be able
// to read the services the other one installed.
func TestCreateServiceRecordWritesOnyxiaApiFormat(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewClientset()
	gw := NewOnyxiaSecretGtw(cs)

	err := gw.CreateServiceRecord(ctx, "user-alice", ports.ServiceRecord{
		ReleaseID:    "jupyter-python-721817",
		CatalogID:    "ide",
		FriendlyName: "My Jupyter",
		Owner:        "alice",
		Share:        true,
	})
	require.NoError(t, err)

	got, err := cs.CoreV1().Secrets("user-alice").
		Get(ctx, "sh.onyxia.release.v1.jupyter-python-721817", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, corev1.SecretType("onyxia.sh/release.v1"), got.Type)
	assert.Equal(t, map[string][]byte{
		"catalog":      []byte("ide"),
		"friendlyName": []byte("My Jupyter"),
		"owner":        []byte("alice"),
		"share":        []byte("true"),
	}, got.Data)
}

func TestCreateServiceRecordReturnsAlreadyExists(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewClientset()
	gw := NewOnyxiaSecretGtw(cs)
	seedSecret(t, cs, "ns", buildOnyxiaSecretName("rel"), "other", map[string][]byte{"owner": []byte("old")})

	err := gw.CreateServiceRecord(ctx, "ns", ports.ServiceRecord{ReleaseID: "rel", Owner: "alice"})
	require.ErrorIs(t, err, domain.ErrAlreadyExists)

	got, err := cs.CoreV1().Secrets("ns").Get(ctx, buildOnyxiaSecretName("rel"), metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, map[string][]byte{"owner": []byte("old")}, got.Data, "existing record must not be overwritten")
}

func TestGetServiceRecord(t *testing.T) {
	cs := k8sfake.NewClientset()
	gw := NewOnyxiaSecretGtw(cs)
	seedSecret(t, cs, "ns", buildOnyxiaSecretName("rel"), onyxiaSecretType, map[string][]byte{
		"catalog": []byte("ide"), "friendlyName": []byte("F"), "owner": []byte("bob"), "share": []byte("true"),
	})

	rec, err := gw.GetServiceRecord(context.Background(), "ns", "rel")

	require.NoError(t, err)
	assert.Equal(t, ports.ServiceRecord{
		ReleaseID: "rel", CatalogID: "ide", FriendlyName: "F", Owner: "bob", Share: true,
	}, rec)
}

func TestGetServiceRecordWithoutDataIsEmpty(t *testing.T) {
	cs := k8sfake.NewClientset()
	gw := NewOnyxiaSecretGtw(cs)
	seedSecret(t, cs, "ns", buildOnyxiaSecretName("rel"), onyxiaSecretType, nil)

	rec, err := gw.GetServiceRecord(context.Background(), "ns", "rel")

	require.NoError(t, err)
	assert.Equal(t, ports.ServiceRecord{ReleaseID: "rel"}, rec)
}

func TestGetServiceRecordNotFound(t *testing.T) {
	gw := NewOnyxiaSecretGtw(k8sfake.NewClientset())

	_, err := gw.GetServiceRecord(context.Background(), "ns", "missing")

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestListServiceRecordsOnlyReturnsOnyxiaSecrets(t *testing.T) {
	cs := k8sfake.NewClientset()
	gw := NewOnyxiaSecretGtw(cs)
	seedSecret(t, cs, "ns", buildOnyxiaSecretName("a"), onyxiaSecretType, map[string][]byte{"owner": []byte("alice")})
	seedSecret(t, cs, "ns", buildOnyxiaSecretName("b"), onyxiaSecretType, map[string][]byte{"owner": []byte("bob")})
	seedSecret(t, cs, "ns", "unrelated", corev1.SecretTypeOpaque, nil)
	seedSecret(t, cs, "other-ns", buildOnyxiaSecretName("c"), onyxiaSecretType, nil)

	var fieldSelector string
	cs.PrependReactor("list", "secrets", func(a k8stesting.Action) (bool, runtime.Object, error) {
		fieldSelector = a.(k8stesting.ListAction).GetListRestrictions().Fields.String()
		return false, nil, nil
	})

	recs, err := gw.ListServiceRecords(context.Background(), "ns")

	require.NoError(t, err)
	assert.ElementsMatch(t, []ports.ServiceRecord{
		{ReleaseID: "a", Owner: "alice"},
		{ReleaseID: "b", Owner: "bob"},
	}, recs)
	assert.Equal(t, "type=onyxia.sh/release.v1", fieldSelector)
}

func TestSetServiceSharedKeepsOtherKeysAndRetriesOnConflict(t *testing.T) {
	ctx := context.Background()
	cs := k8sfake.NewClientset()
	gw := NewOnyxiaSecretGtw(cs)
	seedSecret(t, cs, "ns", buildOnyxiaSecretName("rel"), onyxiaSecretType, map[string][]byte{
		"owner": []byte("alice"), "share": []byte("false"),
	})

	conflictedOnce := false
	cs.PrependReactor("update", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
		if !conflictedOnce {
			conflictedOnce = true
			return true, nil, apierrors.NewConflict(
				schema.GroupResource{Resource: "secrets"}, buildOnyxiaSecretName("rel"), nil)
		}
		return false, nil, nil
	})

	require.NoError(t, gw.SetServiceShared(ctx, "ns", "rel", true))

	got, err := cs.CoreV1().Secrets("ns").Get(ctx, buildOnyxiaSecretName("rel"), metav1.GetOptions{})
	require.NoError(t, err)
	assert.True(t, conflictedOnce)
	assert.Equal(t, map[string][]byte{"owner": []byte("alice"), "share": []byte("true")}, got.Data)
}

func TestSetServiceSharedNotFound(t *testing.T) {
	gw := NewOnyxiaSecretGtw(k8sfake.NewClientset())

	err := gw.SetServiceShared(context.Background(), "ns", "missing", true)

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestDeleteServiceRecordIgnoresNotFound(t *testing.T) {
	gw := NewOnyxiaSecretGtw(k8sfake.NewClientset())

	require.NoError(t, gw.DeleteServiceRecord(context.Background(), "ns", "missing"))
}
