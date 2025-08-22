package ports

import "context"

type OnyxiaSecretGateway interface {
	EnsureOnyxiaSecret(ctx context.Context, namespace, name string, data map[string][]byte) error
	DeleteOnyxiaSecret(ctx context.Context, namespace, name string) error
	ReadOnyxiaSecretData(ctx context.Context, namespace, name string) (map[string][]byte, error)
}
