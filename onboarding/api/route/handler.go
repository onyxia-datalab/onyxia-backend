package route

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/api/controller"
	oas "github.com/onyxia-datalab/onyxia-backend/onboarding/api/oas"
)

type Handler struct {
	onboarding *controller.OnboardingController
}

func NewHandler(
	onboarding *controller.OnboardingController) *Handler {
	return &Handler{onboarding: onboarding}
}

func (h *Handler) Onboard(
	ctx context.Context,
	req *oas.OnboardingRequest,
) (oas.OnboardRes, error) {
	return h.onboarding.Onboard(ctx, req)
}

var _ oas.Handler = (*Handler)(nil)
