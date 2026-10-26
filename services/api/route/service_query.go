package route

import (
	"github.com/onyxia-datalab/onyxia-backend/services/adapters/k8s"
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/namespace"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/service/query"
)

// SetupServiceReader builds the read side of the services, shared by the
// queries and the event streams.
func SetupServiceReader(
	app *bootstrap.Application,
	releaseGtw ports.ReleaseGateway,
	namespaceAuthz namespace.Authorizer,
) *query.Reader {
	secretGtw := k8s.NewOnyxiaSecretGtw(app.K8sClient.Clientset())
	podGtw := k8s.NewWorkloadStateGtw(app.K8sClient.Clientset())

	return query.NewReader(secretGtw, releaseGtw, podGtw, namespaceAuthz)
}

func SetupServiceQueryController(app *bootstrap.Application, reader *query.Reader) *controller.ServiceQueryController {
	return controller.NewServiceQueryController(reader, app.UserContextReader)
}
