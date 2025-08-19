package usecase

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/interfaces"
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

var _ interfaces.NamespaceService = (*MockNamespaceService)(nil)

func (m *MockNamespaceService) CreateNamespace(
	ctx context.Context,
	name string,
	annotations map[string]string,
	labels map[string]string,
) (interfaces.NamespaceCreationResult, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(interfaces.NamespaceCreationResult), args.Error(1)
}

func (m *MockNamespaceService) ApplyResourceQuotas(
	ctx context.Context,
	namespace string,
	quota *domain.Quota,
) (interfaces.QuotaApplicationResult, error) {
	args := m.Called(ctx, namespace, quota)
	return args.Get(0).(interfaces.QuotaApplicationResult), args.Error(1)
}

// ---------- Usercontext helpers ----------

// Default test user used by most tests.
func defaultTestUser() *usercontext.User {
	return &usercontext.User{
		Username: testUserName,
		Groups:   []string{testGroupName},
		Roles:    []string{"role1"},
		Attributes: map[string]any{
			"attr1": "value1",
		},
	}
}

// Returns (ctxWithUser, reader) you can use in tests.
func newCtxAndReaderWithUser(u *usercontext.User) (context.Context, usercontext.Reader) {
	reader, writer := usercontext.NewUserContext()
	ctx := writer.WithUser(context.Background(), u)
	return ctx, reader
}

// ---------- Usecase builders ----------

// Public constructor used by tests that don’t need private fields.
func setupUsecase(
	mockService *MockNamespaceService,
	quotas domain.Quotas,
) domain.OnboardingUsecase {
	_, reader := // keep both for clarity, but only reader is needed here
		func() (context.Context, usercontext.Reader) { return newCtxAndReaderWithUser(defaultTestUser()) }()

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
		reader, // <- shared usercontext.Reader
	)
}

// Private usecase (struct) for tests that reach unexported methods.
func setupPrivateUsecase(
	mockService *MockNamespaceService,
	quotas domain.Quotas,
) *onboardingUsecase {
	_, reader := newCtxAndReaderWithUser(defaultTestUser())

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
		userContextReader: reader, // <- inject the shared Reader
	}
}
