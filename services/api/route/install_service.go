package route

import (
	"fmt"

	"github.com/onyxia-datalab/onyxia-backend/services/adapters/helm"
	"github.com/onyxia-datalab/onyxia-backend/services/adapters/k8s"
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/service/lifecycle"
)

func SetupInstallController(
	app *bootstrap.Application,
	helmClient *helm.Client,
) (*controller.InstallController, error) {

	helmRealeaseGtw, err := helm.NewReleaseGtw(
		app.K8sClient.Config(),
		helmClient,
		ports.InstallCallbacks{},
	)

	if err != nil {
		return nil, fmt.Errorf("helm adapter: %w", err)
	}

	pkgRepo, err := helm.NewPackageRepository(app.Env.CatalogsConfig, helmClient)
	if err != nil {
		return nil, err
	}

	serviceLifecycleUc := lifecycle.NewLifecycle(
		k8s.NewOnyxiaSecretGtw(app.K8sClient.Clientset()),
		helmRealeaseGtw,
		pkgRepo,
	)

	ctrl := controller.NewInstallController(serviceLifecycleUc, app.UserContextReader)

	return ctrl, nil

}
