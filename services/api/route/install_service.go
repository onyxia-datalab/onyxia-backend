package route

import (
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

func SetupInstallController(
	app *bootstrap.Application,
) *controller.InstallController {

	var serviceLifecycleUc domain.ServiceLifecycle // usecase.NewInstallUsecase()

	ctrl := controller.NewInstallController(serviceLifecycleUc, app.UserContextReader)

	return ctrl

}
