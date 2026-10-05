package controller

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

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

	// Extract optional value from OptString
	var groupPtr *string
	if req.Group.Set { // Check if value is set
		groupPtr = &req.Group.Value

		// Check if the requested group is in user's groups
		if !slices.Contains(user.Groups, *groupPtr) {
			slog.ErrorContext(ctx, "Unauthorized group access",
				slog.String("group", *groupPtr),
				slog.Any("userGroups", user.Groups),
			)
			return nil, fmt.Errorf("%w: user does not have access to group: %s", domain.ErrForbidden, *groupPtr)
		}
	}

	if err := c.OnboardingUsecase.Onboard(ctx, domain.OnboardingRequest{
		Group:     groupPtr,
		UserName:  user.Username,
		UserRoles: user.Roles,
	}); err != nil {
		slog.ErrorContext(ctx, "Onboarding failed", slog.Any("error", err))
		return nil, err
	}

	slog.InfoContext(ctx, "Onboarding successful")
	return &api.OnboardOK{}, nil
}
