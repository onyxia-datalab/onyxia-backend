package route

import (
	"context"
	"net/http"

	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	api "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
)

type Handler struct {
	install       *controller.InstallController
	catalogs      *controller.CatalogController
	serviceQuery  *controller.ServiceQueryController
	serviceEvents *controller.ServiceEventsController
}

var (
	_ api.Handler    = (*Handler)(nil)
	_ api.RawHandler = (*Handler)(nil)
)

func NewHandler(
	install *controller.InstallController,
	catalogs *controller.CatalogController,
	serviceQuery *controller.ServiceQueryController,
	serviceEvents *controller.ServiceEventsController,
) *Handler {
	return &Handler{install: install, catalogs: catalogs, serviceQuery: serviceQuery, serviceEvents: serviceEvents}
}

func (h *Handler) SetServiceSuspended(
	ctx context.Context,
	req *api.SetServiceSuspendedReq,
	p api.SetServiceSuspendedParams,
) (api.SetServiceSuspendedRes, error) {
	return h.install.SetServiceSuspended(ctx, req, p)
}

func (h *Handler) SetServiceShared(
	ctx context.Context,
	req *api.SetServiceSharedReq,
	p api.SetServiceSharedParams,
) (api.SetServiceSharedRes, error) {
	return h.install.SetServiceShared(ctx, req, p)
}

func (h *Handler) DeleteService(
	ctx context.Context,
	p api.DeleteServiceParams,
) (api.DeleteServiceRes, error) {
	return h.install.DeleteService(ctx, p)
}

func (h *Handler) InstallService(
	ctx context.Context,
	req *api.ServiceInstallRequest,
	p api.InstallServiceParams,
) (api.InstallServiceRes, error) {
	return h.install.InstallService(ctx, req, p)
}

func (h *Handler) GetService(
	ctx context.Context,
	p api.GetServiceParams,
) (api.GetServiceRes, error) {
	return h.serviceQuery.GetService(ctx, p)
}

func (h *Handler) ListServices(
	ctx context.Context,
	p api.ListServicesParams,
) (api.ListServicesRes, error) {
	return h.serviceQuery.ListServices(ctx, p)
}

func (h *Handler) GetMyCatalogs(ctx context.Context) (api.GetMyCatalogsRes, error) {
	return h.catalogs.GetMyCatalogs(ctx)
}

func (h *Handler) WatchServiceEvents(
	ctx context.Context,
	p api.WatchServiceEventsParams,
	w http.ResponseWriter,
) error {
	return h.serviceEvents.WatchServiceEvents(ctx, p, w)
}

func (h *Handler) GetProjectQuota(
	ctx context.Context,
	p api.GetProjectQuotaParams,
) (api.GetProjectQuotaRes, error) {
	return h.serviceEvents.GetProjectQuota(ctx, p)
}

func (h *Handler) GetMyPackage(
	ctx context.Context,
	p api.GetMyPackageParams,
) (api.GetMyPackageRes, error) {
	return h.catalogs.GetMyPackage(ctx, p.CatalogId, p.PackageName)
}
func (h *Handler) GetPackageSchema(
	ctx context.Context,
	p api.GetPackageSchemaParams,
) (api.GetPackageSchemaRes, error) {
	return h.catalogs.GetPackageSchema(ctx, p.CatalogId, p.PackageName, p.Version)
}
