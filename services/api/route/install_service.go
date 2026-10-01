package route

import (
	"fmt"

	"github.com/onyxia-datalab/onyxia-backend/services/adapters/helm"
	"github.com/onyxia-datalab/onyxia-backend/services/adapters/k8s"
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/namespace"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/service/lifecycle"
)

func SetupInstallController(
	app *bootstrap.Application,
	helmClient *helm.Client,
	catalogUc domain.CatalogService,
	namespaceAuthz namespace.Authorizer,
) (*controller.InstallController, error) {

	helmRealeaseGtw, err := helm.NewReleaseGtw(
		app.K8sClient.Config(),
		helmClient,
		ports.InstallCallbacks{},
	)

	if err != nil {
		return nil, fmt.Errorf("helm adapter: %w", err)
	}

	serviceLifecycleUc := lifecycle.NewLifecycle(
		k8s.NewOnyxiaSecretGtw(app.K8sClient.Clientset()),
		helmRealeaseGtw,
		catalogUc,
		namespaceAuthz,
	)

	ctrl := controller.NewInstallController(serviceLifecycleUc, app.UserContextReader, namespaceAuthz)

	return ctrl, nil

}
