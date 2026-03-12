package route

import (
	"fmt"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/services/adapters/helm"
	"github.com/onyxia-datalab/onyxia-backend/services/adapters/k8s"
	"github.com/onyxia-datalab/onyxia-backend/services/api/controller"
	"github.com/onyxia-datalab/onyxia-backend/services/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase"
)

func SetupInstallController(
	app *bootstrap.Application,
) (*controller.InstallController, error) {

	//TODO: pass callbacks properly
	helmRealeaseGtw, err := helm.NewReleaseGtw(app.K8sClient.Config(), ports.HelmStartCallbacks{
		OnStart: func(release, chart string) {
			slog.Info("Helm install started",
				slog.String("release", release),
				slog.String("chart", chart),
			)
		},
		OnSuccess: func(release, chart string) {
			slog.Info("Helm install succeeded",
				slog.String("release", release),
				slog.String("chart", chart),
			)
		},
		OnError: func(release, chart string, err error) {
			slog.Error("Helm install failed",
				slog.String("release", release),
				slog.String("chart", chart),
				slog.Any("error", err),
			)
		},
	})

	if err != nil {
		return nil, fmt.Errorf("helm adapter: %w", err)
	}

	serviceLifecycleUc := usecase.NewServiceLifecycle(
		k8s.NewOnyxiaSecretGtw(app.K8sClient.Clientset()),
		helmRealeaseGtw,
		helm.NewPackageResolver(app.Env.CatalogsConfig),
	)

	ctrl := controller.NewInstallController(serviceLifecycleUc, app.UserContextReader)

	return ctrl, nil

}
