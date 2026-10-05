package route

import (
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

func SetupCatalogController(catalogUc ports.CatalogService, app *bootstrap.Application) *controller.CatalogController {
	return controller.NewCatalogController(catalogUc, app.UserContextReader)
}
