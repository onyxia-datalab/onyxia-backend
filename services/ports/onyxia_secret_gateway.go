package ports

import "context"

type OnyxiaSecretGateway interface {
	// CreateOnyxiaSecret creates the service record once. It returns
	// domain.ErrAlreadyExists when another installation has already reserved
	// the same release name.
	CreateOnyxiaSecret(ctx context.Context, namespace, name string, data map[string][]byte) error
	// UpdateOnyxiaSecret sets the given keys on an existing service record,
	// leaving the other keys untouched. It returns domain.ErrNotFound when the
	// record does not exist. Callers must authorize the update before invoking
	// it; installation must use Create.
	UpdateOnyxiaSecret(ctx context.Context, namespace, name string, data map[string][]byte) error
	DeleteOnyxiaSecret(ctx context.Context, namespace, name string) error
	// ReadOnyxiaSecretData returns domain.ErrNotFound when the secret does not exist.
	ReadOnyxiaSecretData(ctx context.Context, namespace, name string) (map[string][]byte, error)
	// ListOnyxiaSecretNames returns the release IDs of all Onyxia secrets in the namespace.
	ListOnyxiaSecretNames(ctx context.Context, namespace string) ([]string, error)
}
