package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
)

type ResourceApplier interface {
	// CheckResource fails when the resource cannot be applied in a
	// namespace: unknown kind, or a kind that is not namespaced.
	CheckResource(resource domain.Resource) error
	// ApplyResource creates the resource in namespace, or brings the fields
	// it sets back to their configured values.
	ApplyResource(ctx context.Context, namespace string, resource domain.Resource) error
}
