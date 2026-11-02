package route

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/services/adapters/k8s"
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/namespace"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/service/query"
)

// SetupServiceEventsController wires the event streams and the project
// quota. lifetime ends every stream when it is done (at shutdown).
func SetupServiceEventsController(
	lifetime context.Context,
	app *bootstrap.Application,
	reader *query.Reader,
	namespaceAuthz namespace.Authorizer,
) *controller.ServiceEventsController {
	clientset := app.K8sClient.Clientset()
	quotaGtw := k8s.NewQuotaGtw(clientset)

	streamer := query.NewStreamer(
		lifetime,
		reader,
		quotaGtw,
		k8s.NewClusterWatchGtw(clientset),
		query.DefaultStreamSettings,
	)
	quotas := query.NewQuotaReader(quotaGtw, namespaceAuthz)

	return controller.NewServiceEventsController(streamer, quotas, app.UserContextReader)
}
