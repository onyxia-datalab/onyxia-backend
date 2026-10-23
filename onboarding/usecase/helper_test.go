package usecase

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/ports"
)

// ---------- Shared Test Constants ----------
const (
	testUserName       = "test-user"
	testGroupName      = "test-group"
	defaultNamespace   = "user-test-user"
	userNamespace      = "user-test-user"
	groupNamespace     = "projet-test-group"
	namespacePrefix    = "user-"
	groupNamespacePref = "projet-"
)

// ---------- NamespaceService mock ----------
type MockNamespaceService struct{ mock.Mock }

var _ ports.NamespaceService = (*MockNamespaceService)(nil)

func (m *MockNamespaceService) CreateNamespace(
	ctx context.Context,
	name string,
	annotations map[string]string,
	labels map[string]string,
) (ports.NamespaceCreationResult, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(ports.NamespaceCreationResult), args.Error(1)
}

func (m *MockNamespaceService) ApplyResourceQuotas(
	ctx context.Context,
	namespace string,
	quota *domain.Quota,
) (ports.QuotaApplicationResult, error) {
	args := m.Called(ctx, namespace, quota)
	return args.Get(0).(ports.QuotaApplicationResult), args.Error(1)
}

// ---------- ResourceApplier mock ----------
type MockResourceApplier struct{ mock.Mock }

var _ ports.ResourceApplier = (*MockResourceApplier)(nil)

func (m *MockResourceApplier) CheckResource(resource domain.Resource) error {
	return m.Called(resource).Error(0)
}

func (m *MockResourceApplier) ApplyResource(
	ctx context.Context,
	namespace string,
	resource domain.Resource,
) error {
	return m.Called(ctx, namespace, resource).Error(0)
}

// ---------- Usercontext helpers ----------

var defaultTestUser = &usercontext.User{
	Username: testUserName,
	Groups:   []string{testGroupName},
	Roles:    []string{"role1"},
	Attributes: map[string]any{
		"attr1": "value1",
	},
}

// ---------- Usecase builders ----------

// Public constructor used by tests that don’t need private fields.
func setupUsecase(
	mockService *MockNamespaceService,
	quotas domain.Quotas,
) ports.OnboardingUsecase {

	return NewOnboardingUsecase(
		mockService,
		new(MockResourceApplier),
		domain.Namespace{
			NamespacePrefix:      namespacePrefix,
			GroupNamespacePrefix: groupNamespacePref,
			NamespaceLabels:      nil,
			Annotation: domain.Annotation{
				Enabled: false,
				Static:  nil,
			},
		},
		quotas,
		nil,
	)
}

func setupPrivateUsecase(
	mockService *MockNamespaceService,
	quotas domain.Quotas,
) *onboardingUsecase {
	return &onboardingUsecase{
		namespaceService: mockService,
		namespace: domain.Namespace{
			NamespacePrefix:      namespacePrefix,
			GroupNamespacePrefix: groupNamespacePref,
			Annotation: domain.Annotation{
				Enabled: false,
				Static:  nil,
			},
		},
		quotas: quotas,
	}
}
