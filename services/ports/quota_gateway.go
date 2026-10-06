package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

// QuotaGateway reads the resource quota of a namespace.
type QuotaGateway interface {
	// GetProjectQuota returns the quota of the namespace and its usage. When
	// several quotas limit the same resource, the most restrictive one is
	// returned. A namespace without quota yields an empty ProjectQuota.
	GetProjectQuota(ctx context.Context, namespace string) (domain.ProjectQuota, error)
}
