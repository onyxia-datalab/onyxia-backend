package route

import (
	"github.com/onyxia-datalab/onyxia-backend/services/adapters/k8s"
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/namespace"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/service/lifecycle"
)

func SetupInstallController(
	app *bootstrap.Application,
	releaseGtw ports.ReleaseGateway,
	catalogUc ports.CatalogService,
	namespaceAuthz namespace.Authorizer,
) *controller.InstallController {
	serviceLifecycleUc := lifecycle.NewLifecycle(
		k8s.NewOnyxiaSecretGtw(app.K8sClient.Clientset()),
		releaseGtw,
		catalogUc,
		namespaceAuthz,
	)

	return controller.NewInstallController(serviceLifecycleUc, app.UserContextReader, namespaceAuthz)
}
