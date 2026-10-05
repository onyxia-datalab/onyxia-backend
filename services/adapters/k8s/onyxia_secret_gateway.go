package k8s

import (
	"context"
	"strconv"
	"strings"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

// Service records are stored as "Onyxia secrets", in the format onyxia-api
// (Java) uses, so both APIs can read the services the other installed.
const onyxiaSecretType = corev1.SecretType("onyxia.sh/release.v1")
const onyxiaNamePrefix = "sh.onyxia.release.v1."

const (
	keyCatalog      = "catalog"
	keyFriendlyName = "friendlyName"
	keyOwner        = "owner"
	keyShare        = "share"
)

func buildOnyxiaSecretName(releaseName string) string {
	return onyxiaNamePrefix + releaseName
}

func encodeRecord(rec ports.ServiceRecord) map[string][]byte {
	return map[string][]byte{
		keyCatalog:      []byte(rec.CatalogID),
		keyFriendlyName: []byte(rec.FriendlyName),
		keyOwner:        []byte(rec.Owner),
		keyShare:        []byte(strconv.FormatBool(rec.Share)),
	}
}

func decodeRecord(releaseID string, data map[string][]byte) ports.ServiceRecord {
	return ports.ServiceRecord{
		ReleaseID:    releaseID,
		CatalogID:    string(data[keyCatalog]),
		FriendlyName: string(data[keyFriendlyName]),
		Owner:        string(data[keyOwner]),
		Share:        string(data[keyShare]) == "true",
	}
}

var _ ports.ServiceRecordGateway = (*K8sOnyxiaSecretGateway)(nil)

type K8sOnyxiaSecretGateway struct {
	client kubernetes.Interface
}

func NewOnyxiaSecretGtw(client kubernetes.Interface) *K8sOnyxiaSecretGateway {
	return &K8sOnyxiaSecretGateway{client: client}
}

func (g *K8sOnyxiaSecretGateway) CreateServiceRecord(
	ctx context.Context,
	namespace string,
	rec ports.ServiceRecord,
) error {
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildOnyxiaSecretName(rec.ReleaseID),
			Namespace: namespace,
		},
		Type: onyxiaSecretType,
		Data: encodeRecord(rec),
	}

	_, err := g.client.CoreV1().Secrets(namespace).Create(ctx, sec, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return domain.ErrAlreadyExists
	}
	return err
}

func (g *K8sOnyxiaSecretGateway) GetServiceRecord(
	ctx context.Context,
	namespace, releaseID string,
) (ports.ServiceRecord, error) {
	sec, err := g.client.CoreV1().
		Secrets(namespace).
		Get(ctx, buildOnyxiaSecretName(releaseID), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ports.ServiceRecord{}, domain.ErrNotFound
		}
		return ports.ServiceRecord{}, err
	}
	return decodeRecord(releaseID, sec.Data), nil
}

func (g *K8sOnyxiaSecretGateway) ListServiceRecords(
	ctx context.Context,
	namespace string,
) ([]ports.ServiceRecord, error) {
	list, err := g.client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("type", string(onyxiaSecretType)).String(),
	})
	if err != nil {
		return nil, err
	}

	var records []ports.ServiceRecord
	for _, sec := range list.Items {
		// The field selector already filters on the type server-side; check
		// again for clients that ignore it (e.g. the fake clientset).
		if sec.Type == onyxiaSecretType && strings.HasPrefix(sec.Name, onyxiaNamePrefix) {
			records = append(records, decodeRecord(strings.TrimPrefix(sec.Name, onyxiaNamePrefix), sec.Data))
		}
	}
	return records, nil
}

func (g *K8sOnyxiaSecretGateway) SetServiceShared(
	ctx context.Context,
	namespace, releaseID string,
	share bool,
) error {
	fullName := buildOnyxiaSecretName(releaseID)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cur, getErr := g.client.CoreV1().Secrets(namespace).Get(ctx, fullName, metav1.GetOptions{})
		if getErr != nil {
			if apierrors.IsNotFound(getErr) {
				return domain.ErrNotFound
			}
			return getErr
		}
		if cur.Data == nil {
			cur.Data = map[string][]byte{}
		}
		cur.Data[keyShare] = []byte(strconv.FormatBool(share))

		_, updErr := g.client.CoreV1().Secrets(namespace).Update(ctx, cur, metav1.UpdateOptions{})
		return updErr
	})
}

func (g *K8sOnyxiaSecretGateway) DeleteServiceRecord(
	ctx context.Context,
	namespace, releaseID string,
) error {
	err := g.client.CoreV1().
		Secrets(namespace).
		Delete(ctx, buildOnyxiaSecretName(releaseID), metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
