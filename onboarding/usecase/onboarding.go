package usecase

import (
	"context"
	"fmt"
	"slices"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/ports"
)

type onboardingUsecase struct {
	namespaceService ports.NamespaceService
	namespace        domain.Namespace
	quotas           domain.Quotas
}

func NewOnboardingUsecase(
	namespaceService ports.NamespaceService,
	namespace domain.Namespace,
	quotas domain.Quotas,
) *onboardingUsecase {
	return &onboardingUsecase{
		namespaceService: namespaceService,
		namespace:        namespace,
		quotas:           quotas,
	}
}

// Onboard creates the namespace and applies its quota. A group namespace can
// only be onboarded by a member of the group (ErrForbidden otherwise).
func (s *onboardingUsecase) Onboard(ctx context.Context, req domain.OnboardingRequest) error {
	if req.Group != nil && !slices.Contains(req.User.Groups, *req.Group) {
		return fmt.Errorf("%w: user %q is not a member of group %q", domain.ErrForbidden, req.User.Username, *req.Group)
	}

	namespace := s.getNamespace(req)

	if err := s.createNamespace(ctx, namespace, req.User.Attributes); err != nil {
		return err
	}

	if err := s.applyQuotas(ctx, namespace, req); err != nil {
		return err
	}

	return nil
}

func (s *onboardingUsecase) getNamespace(req domain.OnboardingRequest) string {
	if req.Group != nil {
		return s.namespace.GroupNamespacePrefix + *req.Group
	}
	return s.namespace.NamespacePrefix + req.User.Username
}
