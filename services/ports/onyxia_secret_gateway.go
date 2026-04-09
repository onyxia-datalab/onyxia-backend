package ports

import "context"

type OnyxiaSecretGateway interface {
	EnsureOnyxiaSecret(ctx context.Context, namespace, name string, data map[string][]byte) error
	DeleteOnyxiaSecret(ctx context.Context, namespace, name string) error
	// ReadOnyxiaSecretData returns domain.ErrNotFound when the secret does not exist.
	ReadOnyxiaSecretData(ctx context.Context, namespace, name string) (map[string][]byte, error)
	// ListOnyxiaSecretNames returns the release IDs of all Onyxia secrets in the namespace.
	ListOnyxiaSecretNames(ctx context.Context, namespace string) ([]string, error)
}
