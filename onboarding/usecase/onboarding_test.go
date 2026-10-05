package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ✅ Test `Onboard` Success (Namespace & Quota Applied)
func TestOnboardSuccess(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{
		Enabled:      true,
		GroupEnabled: true,
		Group:        domain.Quota{MemoryRequest: "12Gi"},
	}
	usecase := setupUsecase(mockService, quotas)

	mockService.On("CreateNamespace", mock.Anything, groupNamespace).
		Return(ports.NamespaceCreated, nil)

	mockService.On("ApplyResourceQuotas", mock.Anything, groupNamespace, &quotas.Group).
		Return(ports.QuotaCreated, nil)

	groupName := testGroupName
	req := domain.OnboardingRequest{Group: &groupName, User: *defaultTestUser}
	err := usecase.Onboard(context.Background(), req)

	assert.NoError(t, err)
	mockService.AssertCalled(t, "CreateNamespace", mock.Anything, groupNamespace)
	mockService.AssertCalled(t, "ApplyResourceQuotas", mock.Anything, groupNamespace, &quotas.Group)
}

// ✅ Test `Onboard` Success (Quotas Disabled)
func TestOnboardQuotasDisabled(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{Enabled: false}
	usecase := setupUsecase(mockService, quotas)

	mockService.On("CreateNamespace", mock.Anything, defaultNamespace).
		Return(ports.NamespaceCreated, nil)

	req := domain.OnboardingRequest{Group: nil, User: *defaultTestUser}
	err := usecase.Onboard(context.Background(), req)

	assert.NoError(t, err)
	mockService.AssertCalled(t, "CreateNamespace", mock.Anything, defaultNamespace)
	mockService.AssertNotCalled(t, "ApplyResourceQuotas")
}

// ❌ Test `Onboard` (Namespace Creation Fails)
func TestOnboardCreateNamespaceFails(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{Enabled: true}
	usecase := setupUsecase(mockService, quotas)

	expectedError := errors.New("namespace creation failed")
	mockService.On("CreateNamespace", mock.Anything, groupNamespace).
		Return(ports.NamespaceCreationResult(""), expectedError)

	groupName := testGroupName
	req := domain.OnboardingRequest{Group: &groupName, User: *defaultTestUser}
	err := usecase.Onboard(context.Background(), req)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	mockService.AssertCalled(t, "CreateNamespace", mock.Anything, groupNamespace)
	mockService.AssertNotCalled(t, "ApplyResourceQuotas")
}

// ❌ Test `Onboard` (Quota Application Fails)
func TestOnboardApplyResourceQuotasFails(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{Enabled: true, Default: domain.Quota{MemoryRequest: "10Gi"}}
	usecase := setupUsecase(mockService, quotas)

	mockService.On("CreateNamespace", mock.Anything, defaultNamespace).
		Return(ports.NamespaceCreated, nil)

	mockService.On("ApplyResourceQuotas", mock.Anything, defaultNamespace, &quotas.Default).
		Return(ports.QuotaApplicationResult(""), errors.New("failed to apply quota"))

	req := domain.OnboardingRequest{Group: nil, User: *defaultTestUser}
	err := usecase.Onboard(context.Background(), req)

	assert.Error(t, err)
	mockService.AssertCalled(t, "CreateNamespace", mock.Anything, defaultNamespace)
	mockService.AssertCalled(
		t,
		"ApplyResourceQuotas",
		mock.Anything,
		defaultNamespace,
		&quotas.Default,
	)
}

// ✅ Test `Onboard` Success (Namespace Already Exists)
func TestOnboardNamespaceAlreadyExists(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{Enabled: true, Default: domain.Quota{MemoryRequest: "10Gi"}}
	usecase := setupUsecase(mockService, quotas)

	mockService.On("CreateNamespace", mock.Anything, defaultNamespace).
		Return(ports.NamespaceAlreadyExists, nil)

	mockService.On("ApplyResourceQuotas", mock.Anything, defaultNamespace, &quotas.Default).
		Return(ports.QuotaCreated, nil)

	req := domain.OnboardingRequest{Group: nil, User: *defaultTestUser}
	err := usecase.Onboard(context.Background(), req)

	assert.NoError(t, err)
	mockService.AssertCalled(t, "CreateNamespace", mock.Anything, defaultNamespace)
	mockService.AssertCalled(
		t,
		"ApplyResourceQuotas",
		mock.Anything,
		defaultNamespace,
		&quotas.Default,
	)
}

func TestGetNamespace(t *testing.T) {
	usecase := setupPrivateUsecase(new(MockNamespaceService), domain.Quotas{})

	groupName := testGroupName

	// Case 1: Group is provided
	reqWithGroup := domain.OnboardingRequest{Group: &groupName, User: *defaultTestUser}
	assert.Equal(t, groupNamespace, usecase.getNamespace(reqWithGroup))

	// Case 2: No group, only user
	reqWithoutGroup := domain.OnboardingRequest{Group: nil, User: *defaultTestUser}
	assert.Equal(t, userNamespace, usecase.getNamespace(reqWithoutGroup))
}

func TestOnboardForeignGroupForbidden(t *testing.T) {
	mockService := new(MockNamespaceService)
	usecase := setupUsecase(mockService, domain.Quotas{})
	otherGroup := "not-my-group"

	err := usecase.Onboard(context.Background(), domain.OnboardingRequest{User: *defaultTestUser, Group: &otherGroup})

	assert.ErrorIs(t, err, domain.ErrForbidden)
	mockService.AssertNotCalled(t, "CreateNamespace", mock.Anything, mock.Anything)
}
