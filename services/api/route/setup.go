package route

import (
	"context"
	"fmt"
	"net/http"

	middleware "github.com/onyxia-datalab/onyxia-backend/services/api/middleware"
	oas "github.com/onyxia-datalab/onyxia-backend/services/api/oas"

	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
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

	installCtrl, err := SetupInstallController(app)

	if err != nil {
		return nil, fmt.Errorf("failed to setup install controller: %w", err)
	}

	h := NewHandler(installCtrl)

	srv, err := oas.NewServer(
		h,
		auth,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create api server: %w", err)
	}

	return srv, nil
}
