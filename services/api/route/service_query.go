package route

import (
	"fmt"

	"github.com/onyxia-datalab/onyxia-backend/services/adapters/helm"
	"github.com/onyxia-datalab/onyxia-backend/services/adapters/k8s"
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/namespace"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/service/query"
)

func SetupServiceQueryController(
	app *bootstrap.Application,
	helmClient *helm.Client,
	namespaceAuthz namespace.Authorizer,
) (*controller.ServiceQueryController, error) {

	helmReleaseGtw, err := helm.NewReleaseGtw(
		app.K8sClient.Config(),
		helmClient,
		app.Env.CatalogsConfig,
		ports.InstallCallbacks{},
	)
	if err != nil {
		return nil, fmt.Errorf("helm adapter (query): %w", err)
	}

	secretGtw := k8s.NewOnyxiaSecretGtw(app.K8sClient.Clientset())
	podGtw := k8s.NewWorkloadStateGtw(app.K8sClient.Clientset())

	serviceQueryUc := query.NewReader(secretGtw, helmReleaseGtw, podGtw, app.UserContextReader, namespaceAuthz)

	return controller.NewServiceQueryController(serviceQueryUc, app.UserContextReader, namespaceAuthz), nil
}
