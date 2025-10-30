package route

import (
	"context"

	ht "github.com/ogen-go/ogen/http"
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	api "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
)

type Handler struct {
	install  *controller.InstallController
	catalogs *controller.CatalogController
}

var _ api.Handler = (*Handler)(nil)

func NewHandler(
	install *controller.InstallController,
	catalogs *controller.CatalogController,
) *Handler {
	return &Handler{install: install, catalogs: catalogs}
}

func (h *Handler) InstallService(
	ctx context.Context,
	req *api.ServiceInstallRequest,
	p api.InstallServiceParams,
) (api.InstallServiceRes, error) {
	return h.install.InstallService(ctx, req, p)
}

func (h *Handler) GetMyCatalogs(ctx context.Context) (api.GetMyCatalogsRes, error) {
	return h.catalogs.GetMyCatalogs(ctx)
}

// Keep stubs explicit until implemented (or embed api.UnimplementedHandler if you prefer 501s)
func (h *Handler) WatchRelease(
	ctx context.Context,
	p api.WatchReleaseParams,
) (api.WatchReleaseRes, error) {
	return nil, ht.ErrNotImplemented
}

func (h *Handler) WatchResources(
	ctx context.Context,
	p api.WatchResourcesParams,
) (api.WatchResourcesRes, error) {
	return nil, ht.ErrNotImplemented
}

func (h *Handler) GetMyPackage(
	ctx context.Context,
	p api.GetMyPackageParams,
) (api.GetMyPackageRes, error) {
	return nil, ht.ErrNotImplemented
}
func (h *Handler) APIServicesSchemasCatalogIdPackageNamePackageNameVersionsVersionGet(
	ctx context.Context,
	p api.APIServicesSchemasCatalogIdPackageNamePackageNameVersionsVersionGetParams,
) (api.APIServicesSchemasCatalogIdPackageNamePackageNameVersionsVersionGetRes, error) {
	return nil, ht.ErrNotImplemented
}
