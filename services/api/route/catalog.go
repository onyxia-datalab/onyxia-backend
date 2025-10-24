package route

import (
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase"
)

func SetupCatalogController(app *bootstrap.Application) (*controller.CatalogController, error) {
	catalogUc := usecase.NewCatalogService(app.Env.CatalogsConfig)
	return controller.NewCatalogController(catalogUc, app.UserContextReader), nil
}
