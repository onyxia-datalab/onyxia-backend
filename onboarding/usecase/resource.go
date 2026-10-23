package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
)

// applyResources applies the configured resources in their configured
// order, stopping at the first failure.
func (s *onboardingUsecase) applyResources(
	ctx context.Context,
	namespace string,
	req domain.OnboardingRequest,
) error {
	values := map[string]string{
		domain.PlaceholderNamespace: namespace,
		domain.PlaceholderUsername:  req.User.Username,
	}
	if req.Group != nil {
		values[domain.PlaceholderGroup] = *req.Group
	}

	for i, resource := range s.resources {
		if err := s.resourceApplier.ApplyResource(ctx, namespace, resource.Render(values)); err != nil {
			slog.ErrorContext(ctx, "Failed to apply resource",
				slog.String("namespace", namespace),
				slog.Int("index", i),
				slog.Any("error", err),
			)
			return fmt.Errorf("failed to apply namespaceResources[%d] to namespace (%s): %w", i, namespace, err)
		}
	}

	if len(s.resources) > 0 {
		slog.InfoContext(ctx, "Resources applied",
			slog.String("namespace", namespace),
			slog.Int("count", len(s.resources)),
		)
	}
	return nil
}
