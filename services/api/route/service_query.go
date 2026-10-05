package route

import (
	"github.com/onyxia-datalab/onyxia-backend/services/adapters/k8s"
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/namespace"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/service/query"
)

func SetupServiceQueryController(
	app *bootstrap.Application,
	releaseGtw ports.ReleaseGateway,
	namespaceAuthz namespace.Authorizer,
) *controller.ServiceQueryController {
	secretGtw := k8s.NewOnyxiaSecretGtw(app.K8sClient.Clientset())
	podGtw := k8s.NewWorkloadStateGtw(app.K8sClient.Clientset())

	serviceQueryUc := query.NewReader(secretGtw, releaseGtw, podGtw, app.UserContextReader, namespaceAuthz)

	return controller.NewServiceQueryController(serviceQueryUc, app.UserContextReader, namespaceAuthz)
}
