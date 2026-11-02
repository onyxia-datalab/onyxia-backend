package query

import (
	"context"
	"fmt"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/namespace"
)

type QuotaReader struct {
	quotas     ports.ProjectQuotaReader
	namespaces namespace.Authorizer
}

var _ ports.ProjectQuotaGetter = (*QuotaReader)(nil)

func NewQuotaReader(quotas ports.ProjectQuotaReader, namespaces namespace.Authorizer) *QuotaReader {
	return &QuotaReader{quotas: quotas, namespaces: namespaces}
}

func (uc *QuotaReader) GetProjectQuota(
	ctx context.Context,
	user usercontext.User,
	namespace string,
) (domain.ProjectQuota, error) {
	if err := uc.namespaces.Check(user, namespace); err != nil {
		return domain.ProjectQuota{}, err
	}
	quota, err := uc.quotas.ReadProjectQuota(ctx, namespace)
	if err != nil {
		return domain.ProjectQuota{}, fmt.Errorf("get project quota: %w", err)
	}
	return quota, nil
}
