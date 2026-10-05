package controller

import (
	"context"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	api "github.com/onyxia-datalab/onyxia-backend/onboarding/api/oas"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/ports"
)

type OnboardingController struct {
	OnboardingUsecase ports.OnboardingUsecase
	users             usercontext.UserGetter
}

func NewOnboardingController(
	onboardingUsecase ports.OnboardingUsecase,
	users usercontext.UserGetter,
) *OnboardingController {
	return &OnboardingController{
		OnboardingUsecase: onboardingUsecase,
		users:             users,
	}
}

func (c *OnboardingController) Onboard(
	ctx context.Context,
	req *api.OnboardingRequest,
) (api.OnboardRes, error) {
	slog.InfoContext(ctx, "Received onboarding request")

	user, ok := c.users.GetUser(ctx)
	if !ok || user == nil {
		slog.ErrorContext(ctx, "Failed to retrieve user from context")
		return nil, domain.ErrForbidden
	}

	var group *string
	if req.Group.Set {
		group = &req.Group.Value
	}

	if err := c.OnboardingUsecase.Onboard(ctx, domain.OnboardingRequest{
		User:  *user,
		Group: group,
	}); err != nil {
		slog.ErrorContext(ctx, "Onboarding failed", slog.Any("error", err))
		return nil, err
	}

	slog.InfoContext(ctx, "Onboarding successful")
	return &api.OnboardOK{}, nil
}
