package usecase

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/port"
	"github.com/stretchr/testify/mock"
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

var _ port.NamespaceService = (*MockNamespaceService)(nil)

func (m *MockNamespaceService) CreateNamespace(
	ctx context.Context,
	name string,
	annotations map[string]string,
	labels map[string]string,
) (port.NamespaceCreationResult, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(port.NamespaceCreationResult), args.Error(1)
}

func (m *MockNamespaceService) ApplyResourceQuotas(
	ctx context.Context,
	namespace string,
	quota *domain.Quota,
) (port.QuotaApplicationResult, error) {
	args := m.Called(ctx, namespace, quota)
	return args.Get(0).(port.QuotaApplicationResult), args.Error(1)
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
) domain.OnboardingUsecase {

	_, reader, _ := usercontext.NewTestUserContext(defaultTestUser)

	return NewOnboardingUsecase(
		mockService,
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
		reader,
	)
}

func setupPrivateUsecase(
	mockService *MockNamespaceService,
	quotas domain.Quotas,
) *onboardingUsecase {
	_, reader, _ := usercontext.NewTestUserContext(defaultTestUser)

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
		quotas:            quotas,
		userContextReader: reader,
	}
}
