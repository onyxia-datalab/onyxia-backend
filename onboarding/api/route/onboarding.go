package route

import (
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/adapters/kubernetes"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/api/controller"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/bootstrap"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/ports"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/usecase"
)

func SetupOnboardingController(
	app *bootstrap.Application,
) (*controller.OnboardingController, error) {
	namespaceCreator := kubernetes.NewKubernetesNamespaceService(app.K8sClient.Clientset())

	resourceApplier, resources, err := setupResources(app)
	if err != nil {
		return nil, err
	}

	envQuotas := app.Env.Onboarding.Quotas

	rolesDomainQuotas := func() map[string]domain.Quota {
		result := make(map[string]domain.Quota)
		for key, q := range envQuotas.Roles {
			result[key] = convertBootstrapQuotaToDomain(q)
		}
		return result
	}()

	onboardingUsecase := usecase.NewOnboardingUsecase(
		namespaceCreator,
		resourceApplier,
		domain.Namespace{
			NamespacePrefix:      app.Env.Onboarding.NamespacePrefix,
			GroupNamespacePrefix: app.Env.Onboarding.GroupNamespacePrefix,
			NamespaceLabels:      app.Env.Onboarding.NamespaceLabels,
			Annotation: domain.Annotation{
				Enabled: app.Env.Onboarding.Annotation.Enabled,
				Static:  app.Env.Onboarding.Annotation.Static,
				Dynamic: struct {
					LastLoginTimestamp bool
					UserAttributes     []string
				}(app.Env.Onboarding.Annotation.Dynamic),
			},
		},
		domain.Quotas{
			Enabled:      envQuotas.Enabled,
			Default:      convertBootstrapQuotaToDomain(envQuotas.Default),
			UserEnabled:  envQuotas.UserEnabled,
			User:         convertBootstrapQuotaToDomain(envQuotas.User),
			Roles:        rolesDomainQuotas,
			GroupEnabled: envQuotas.GroupEnabled,
			Group:        convertBootstrapQuotaToDomain(envQuotas.Group),
		},
		resources,
	)

	return controller.NewOnboardingController(onboardingUsecase, app.UserContextReader), nil

}

func convertBootstrapQuotaToDomain(q bootstrap.Quota) domain.Quota {
	return domain.Quota{
		MemoryRequest:           q.RequestsMemory,
		CPURequest:              q.RequestsCPU,
		MemoryLimit:             q.LimitsMemory,
		CPULimit:                q.LimitsCPU,
		StorageRequest:          q.RequestsStorage,
		MaxPods:                 q.CountPods,
		EphemeralStorageRequest: q.RequestsEphemeralStorage,
		EphemeralStorageLimit:   q.LimitsEphemeralStorage,
		GPURequest:              q.RequestsGPU,
		GPULimit:                q.LimitsGPU,
	}
}

// setupResources fails when a configured resource could never be applied,
// so a misconfiguration stops the startup instead of every onboarding.
func setupResources(app *bootstrap.Application) (ports.ResourceApplier, []domain.Resource, error) {
	resources, err := bootstrap.ParseResources(app.Env.Onboarding.NamespaceResources)
	if err != nil {
		return nil, nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(app.K8sClient.Config())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Kubernetes dynamic client: %w", err)
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(app.K8sClient.Config())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Kubernetes discovery client: %w", err)
	}
	// Refreshes its cache on unknown kinds, so CRDs installed after startup
	// are found.
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	applier := kubernetes.NewKubernetesResourceApplier(dynamicClient, mapper)
	for i, resource := range resources {
		if err := applier.CheckResource(resource); err != nil {
			return nil, nil, fmt.Errorf("invalid onboarding.namespaceResources[%d]: %w", i, err)
		}
	}
	return applier, resources, nil
}
