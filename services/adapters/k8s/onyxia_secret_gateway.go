package k8s

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/ports"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const onyxiaSecretType = corev1.SecretType("onyxia.sh/release.v1")
const onyxiaNamePrefix = "sh.onyxia.release.v1."

func buildOnyxiaSecretName(releaseName string) string {
	return onyxiaNamePrefix + releaseName
}

var _ ports.OnyxiaSecretGateway = (*K8sOnyxiaSecretGateway)(nil)

type K8sOnyxiaSecretGateway struct {
	client kubernetes.Interface
}

func NewK8sOnyxiaSecretGateway(client kubernetes.Interface) *K8sOnyxiaSecretGateway {
	return &K8sOnyxiaSecretGateway{client: client}
}

func (g *K8sOnyxiaSecretGateway) EnsureOnyxiaSecret(
	ctx context.Context,
	namespace, name string,
	data map[string][]byte,
) error {

	if data == nil {
		data = map[string][]byte{}
	}

	fullName := buildOnyxiaSecretName(name)

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fullName,
			Namespace: namespace,
		},
		Type: onyxiaSecretType,
		Data: data,
	}

	_, err := g.client.CoreV1().Secrets(namespace).Create(ctx, sec, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return err
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cur, getErr := g.client.CoreV1().Secrets(namespace).Get(ctx, fullName, metav1.GetOptions{})
		if getErr != nil {
			if apierrors.IsNotFound(getErr) {
				_, cErr := g.client.CoreV1().
					Secrets(namespace).
					Create(ctx, sec, metav1.CreateOptions{})
				return cErr
			}
			return getErr
		}
		cur.Type = onyxiaSecretType
		cur.Data = data

		_, updErr := g.client.CoreV1().Secrets(namespace).Update(ctx, cur, metav1.UpdateOptions{})
		return updErr
	})
}

func (g *K8sOnyxiaSecretGateway) DeleteOnyxiaSecret(
	ctx context.Context,
	namespace, name string,
) error {
	err := g.client.CoreV1().
		Secrets(namespace).
		Delete(ctx, buildOnyxiaSecretName(name), metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (g *K8sOnyxiaSecretGateway) ReadOnyxiaSecretData(
	ctx context.Context,
	namespace, name string,
) (map[string][]byte, error) {
	sec, err := g.client.CoreV1().
		Secrets(namespace).
		Get(ctx, buildOnyxiaSecretName(name), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if sec.Data == nil {
		return map[string][]byte{}, nil
	}

	return sec.Data, nil
}
