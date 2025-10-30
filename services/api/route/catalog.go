package route

import (
	"github.com/onyxia-datalab/onyxia-backend/services/adapters/helm"
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase"
	"helm.sh/helm/v3/pkg/cli"
)

func SetupCatalogController(app *bootstrap.Application) (*controller.CatalogController, error) {
	settings := cli.New()

	catalogRepo, err := helm.NewCatalogRepo(settings, app.Env.CatalogsConfig)
	if err != nil {
		return nil, err
	}

	catalogUc := usecase.NewCatalogService(
		app.Env.CatalogsConfig,
		catalogRepo,
		app.UserContextReader,
	)

	return controller.NewCatalogController(catalogUc, app.UserContextReader), nil
}
