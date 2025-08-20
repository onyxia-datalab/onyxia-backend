package route

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/api/controller"
	oas "github.com/onyxia-datalab/onyxia-backend/onboarding/api/oas"
)

type Handler struct {
	onboard *controller.OnboardingController
}

func NewHandler(
	onboard *controller.OnboardingController) *Handler {
	return &Handler{onboard: onboard}
}

func (h *Handler) Onboard(
	ctx context.Context,
	req *oas.OnboardingRequest,
) (oas.OnboardRes, error) {
	return h.onboard.Onboard(ctx, req)
}

var _ oas.Handler = (*Handler)(nil)
