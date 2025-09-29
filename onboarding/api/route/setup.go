package route

import (
	"context"
	"fmt"
	"net/http"

	middleware "github.com/onyxia-datalab/onyxia-backend/onboarding/api/middleware"
	oas "github.com/onyxia-datalab/onyxia-backend/onboarding/api/oas"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/bootstrap"
)

func Setup(ctx context.Context, app *bootstrap.Application) (http.Handler, error) {

	auth, err := middleware.BuildSecurityHandler(ctx,
		app.Env.AuthenticationMode,
		middleware.OIDCConfigOnboarding(app.Env.OIDC),
		app.UserContextWriter,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to initialize OIDC middleware: %w", err)
	}

	onboardingController := SetupOnboardingController(app)

	handler := NewHandler(onboardingController)

	srv, err := oas.NewServer(
		handler,
		auth,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create api server: %w", err)
	}

	return srv, nil
}
