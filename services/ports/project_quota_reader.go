package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

// ProjectQuotaReader reads the resource quota of a namespace.
type ProjectQuotaReader interface {
	// ReadProjectQuota returns the quota of the namespace and its usage. When
	// several quotas limit the same resource, the most restrictive one is
	// returned. A namespace without quota yields an empty ProjectQuota.
	ReadProjectQuota(ctx context.Context, namespace string) (domain.ProjectQuota, error)
}
