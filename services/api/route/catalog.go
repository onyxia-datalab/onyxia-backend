package route

import (
	"github.com/onyxia-datalab/onyxia-backend/services/adapters/helm"
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase"
)

func SetupCatalogController(app *bootstrap.Application) (*controller.CatalogController, error) {

	pkgRepo, err := helm.NewPackageRepository(app.Env.CatalogsConfig)
	if err != nil {
		return nil, err
	}

	catalogUc := usecase.NewCatalogService(
		app.Env.CatalogsConfig,
		pkgRepo,
		app.UserContextReader,
	)

	return controller.NewCatalogController(catalogUc, app.UserContextReader), nil
}
