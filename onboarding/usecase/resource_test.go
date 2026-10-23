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

func setupResourceUsecase(
	mockService *MockNamespaceService,
	mockApplier *MockResourceApplier,
	resources []domain.Resource,
) ports.OnboardingUsecase {
	mockService.On("CreateNamespace", mock.Anything, mock.Anything).
		Return(ports.NamespaceCreated, nil)
	return NewOnboardingUsecase(
		mockService,
		mockApplier,
		domain.Namespace{NamespacePrefix: namespacePrefix, GroupNamespacePrefix: groupNamespacePref},
		domain.Quotas{},
		resources,
	)
}

var testResources = []domain.Resource{
	{"kind": "ServiceAccount", "metadata": map[string]any{"name": "onyxia-installer-sa"}},
	{"kind": "RoleBinding", "subjects": []any{map[string]any{
		"namespace": "${onyxia.namespace}",
		"user":      "${onyxia.username}",
		"group":     "${onyxia.group}",
	}}},
}

func TestOnboardAppliesResourcesInOrder(t *testing.T) {
	mockApplier := new(MockResourceApplier)
	usecase := setupResourceUsecase(new(MockNamespaceService), mockApplier, testResources)

	first := mockApplier.On("ApplyResource", mock.Anything, userNamespace, testResources[0]).Return(nil)
	mockApplier.On("ApplyResource", mock.Anything, userNamespace, domain.Resource{
		"kind": "RoleBinding",
		"subjects": []any{map[string]any{
			"namespace": userNamespace,
			"user":      testUserName,
			"group":     "",
		}},
	}).Return(nil).NotBefore(first)

	err := usecase.Onboard(context.Background(), domain.OnboardingRequest{User: *defaultTestUser})

	assert.NoError(t, err)
	mockApplier.AssertExpectations(t)
}

func TestOnboardRendersGroupPlaceholders(t *testing.T) {
	mockApplier := new(MockResourceApplier)
	usecase := setupResourceUsecase(new(MockNamespaceService), mockApplier, testResources[1:])
	group := testGroupName

	mockApplier.On("ApplyResource", mock.Anything, groupNamespace, domain.Resource{
		"kind": "RoleBinding",
		"subjects": []any{map[string]any{
			"namespace": groupNamespace,
			"user":      testUserName,
			"group":     testGroupName,
		}},
	}).Return(nil)

	err := usecase.Onboard(context.Background(), domain.OnboardingRequest{User: *defaultTestUser, Group: &group})

	assert.NoError(t, err)
	mockApplier.AssertExpectations(t)
}

func TestOnboardWithoutResources(t *testing.T) {
	mockApplier := new(MockResourceApplier)
	usecase := setupResourceUsecase(new(MockNamespaceService), mockApplier, nil)

	err := usecase.Onboard(context.Background(), domain.OnboardingRequest{User: *defaultTestUser})

	assert.NoError(t, err)
	mockApplier.AssertNotCalled(t, "ApplyResource", mock.Anything, mock.Anything, mock.Anything)
}

func TestOnboardStopsAtFirstFailingResource(t *testing.T) {
	mockApplier := new(MockResourceApplier)
	usecase := setupResourceUsecase(new(MockNamespaceService), mockApplier, testResources)

	expectedError := errors.New("forbidden")
	mockApplier.On("ApplyResource", mock.Anything, userNamespace, testResources[0]).Return(expectedError)

	err := usecase.Onboard(context.Background(), domain.OnboardingRequest{User: *defaultTestUser})

	assert.ErrorIs(t, err, expectedError)
	mockApplier.AssertNumberOfCalls(t, "ApplyResource", 1)
}
