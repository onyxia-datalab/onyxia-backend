package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/port"
)

func (s *onboardingUsecase) applyQuotas(
	ctx context.Context,
	namespace string,
	req domain.OnboardingRequest,
) error {
	if !s.quotas.Enabled {
		slog.WarnContext(ctx, "Quotas are disabled, skipping quota application",
			slog.String("namespace", namespace),
		)
		return nil
	}

	quotaToApply := s.getQuota(ctx, req, namespace)

	slog.InfoContext(ctx, "Applying quota",
		slog.Any("quota", quotaToApply),
	)
	result, err := s.namespaceService.ApplyResourceQuotas(ctx, namespace, quotaToApply)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to apply quotas",
			slog.String("namespace", namespace),
			slog.Any("error", err),
		)
		return fmt.Errorf("failed to apply quotas to namespace (%s): %w", namespace, err)
	}

	switch result {
	case port.QuotaCreated:
		slog.InfoContext(ctx, "Resource quota created",
			slog.String("namespace", namespace),
		)
	case port.QuotaUpdated:
		slog.InfoContext(ctx, "Resource quota updated",
			slog.String("namespace", namespace),
		)
	case port.QuotaUnchanged:
		slog.InfoContext(ctx, "Resource quota already up-to-date",
			slog.String("namespace", namespace),
		)
	case port.QuotaIgnored:
		slog.WarnContext(ctx, "Quota ignored due to annotation",
			slog.String("namespace", namespace),
		)
	}

	return nil
}

func (s *onboardingUsecase) getQuota(
	ctx context.Context,
	req domain.OnboardingRequest,
	namespace string,
) *domain.Quota {
	if req.Group != nil {
		return s.getGroupQuota(ctx, req, namespace)
	}
	return s.getUserQuota(ctx, req, namespace)
}

func (s *onboardingUsecase) getGroupQuota(
	ctx context.Context,
	req domain.OnboardingRequest,
	namespace string,
) *domain.Quota {
	if s.quotas.GroupEnabled {
		slog.InfoContext(ctx, "Applying group quota",
			slog.String("namespace", namespace),
			slog.String("group", *req.Group),
		)
		return &s.quotas.Group
	}
	return &s.quotas.Default
}

func (s *onboardingUsecase) getUserQuota(
	ctx context.Context,
	req domain.OnboardingRequest,
	namespace string,
) *domain.Quota {
	for _, role := range req.UserRoles {
		if quota, exists := s.quotas.Roles[role]; exists {
			slog.InfoContext(ctx, "Applying role-based user quota",
				slog.String("namespace", namespace),
				slog.String("role", role),
			)
			return &quota
		}
	}

	if s.quotas.UserEnabled {
		slog.InfoContext(ctx, "Applying user quota",
			slog.String("namespace", namespace),
		)
		return &s.quotas.User
	}

	slog.InfoContext(ctx, "Applying default quota",
		slog.String("namespace", namespace),
	)
	return &s.quotas.Default
}
