package route

import (
	"context"
	"fmt"
	"net/http"

	"github.com/onyxia-datalab/onyxia-backend/internal/apperror"
	"github.com/onyxia-datalab/onyxia-backend/internal/httputil"
	"github.com/onyxia-datalab/onyxia-backend/internal/server"
	"github.com/onyxia-datalab/onyxia-backend/services/adapters/helm"
	middleware "github.com/onyxia-datalab/onyxia-backend/services/api/middleware"
	oas "github.com/onyxia-datalab/onyxia-backend/services/api/oas"

	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap/env"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/catalog"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/namespace"
)

// Setup wires the services API. The returned drain function waits for the
// background work the API started (Helm installs) during a graceful shutdown.
func Setup(ctx context.Context, app *bootstrap.Application) (http.Handler, server.DrainFunc, error) {

	auth, err := middleware.BuildSecurityHandler(ctx,
		app.Env.AuthenticationMode,
		middleware.OIDCConfigOnboarding(app.Env.OIDC),
		app.UserContextWriter,
	)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize OIDC middleware: %w", err)
	}

	helmClient, err := helm.NewClient("")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize helm client: %w", err)
	}

	// A single package repository (and its underlying caches) is shared
	// between the catalog and install paths — building one per controller
	// duplicated the OCI/index cache for no reason.
	pkgRepo, err := helm.NewPackageRepository(app.Env.CatalogsConfig, helmClient)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to setup package repository: %w", err)
	}

	catalogSettings, err := env.CatalogSettings(app.Env.CatalogsConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid catalogs configuration: %w", err)
	}

	catalogUc := catalog.NewCatalogService(
		catalogSettings,
		app.Env.Schemas.Overrides(),
		pkgRepo,
	)

	namespaceAuthz := namespace.NewAuthorizer(
		app.Env.Kubernetes.NamespacePrefix,
		app.Env.Kubernetes.GroupNamespacePrefix,
	)

	// One release gateway for both the install and the query paths, so that
	// the installs it tracks are the ones the shutdown drain waits for.
	releaseGtw := helm.NewReleaseGtw(app.K8sClient.Config(), helmClient, app.Env.CatalogsConfig)

	installCtrl := SetupInstallController(app, releaseGtw, catalogUc, namespaceAuthz)
	catalogCtrl := SetupCatalogController(catalogUc, app)
	reader := SetupServiceReader(app, releaseGtw, namespaceAuthz)
	serviceQueryCtrl := SetupServiceQueryController(app, reader)
	serviceEventsCtrl := SetupServiceEventsController(ctx, app, reader, namespaceAuthz)

	h := NewHandler(installCtrl, catalogCtrl, serviceQueryCtrl, serviceEventsCtrl)

	srv, err := oas.NewServer(
		h,
		auth,
		oas.WithErrorHandler(apperror.OgenHandler),
	)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to create api server: %w", err)
	}

	return httputil.FlushEventStreams(srv), releaseGtw.WaitForInstalls, nil
}
