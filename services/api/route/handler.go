package route

import (
	"context"

	ht "github.com/ogen-go/ogen/http"
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	api "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
)

type Handler struct {
	install *controller.InstallController
}

var _ api.Handler = (*Handler)(nil)

func NewHandler(install *controller.InstallController) *Handler {
	return &Handler{install: install}
}

func (h *Handler) InstallService(
	ctx context.Context,
	req *api.ServiceInstallRequest,
	p api.InstallServiceParams,
) (api.InstallServiceRes, error) {
	return h.install.InstallService(ctx, req, p)
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
