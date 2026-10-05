package ports

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
)

// Driving port: the use case the API layer calls, implemented in usecase/.

type OnboardingUsecase interface {
	Onboard(ctx context.Context, req domain.OnboardingRequest) error
}
