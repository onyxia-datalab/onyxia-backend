// ports/kubernetes.go
package ports

import "context"

type KubernetesService interface {
	CreateOnyxiaSecret(ctx context.Context, namespace, name string, data map[string][]byte) error
}
