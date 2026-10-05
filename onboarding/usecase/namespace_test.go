package usecase

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateNamespaceSuccess(t *testing.T) {
	mockService := new(MockNamespaceService)
	usecase := setupPrivateUsecase(mockService, domain.Quotas{})

	mockService.On(
		"CreateNamespace",
		mock.Anything, // ctx
		userNamespace, // name
		mock.Anything, // annotations
		mock.Anything, // labels
	).Return(ports.NamespaceCreated, nil)

	err := usecase.createNamespace(context.Background(), userNamespace)

	assert.NoError(t, err)
	mockService.AssertCalled(
		t,
		"CreateNamespace",
		mock.Anything,
		userNamespace,
		mock.Anything,
		mock.Anything,
	)
}

func TestCreateNamespaceAlreadyExists(t *testing.T) {
	mockService := new(MockNamespaceService)
	usecase := setupPrivateUsecase(mockService, domain.Quotas{})

	mockService.On(
		"CreateNamespace",
		mock.Anything, userNamespace, mock.Anything, mock.Anything,
	).Return(ports.NamespaceAlreadyExists, nil)

	err := usecase.createNamespace(context.Background(), userNamespace)

	assert.NoError(t, err)
	mockService.AssertCalled(
		t,
		"CreateNamespace",
		mock.Anything,
		userNamespace,
		mock.Anything,
		mock.Anything,
	)
}

func TestCreateNamespaceFailure(t *testing.T) {
	mockService := new(MockNamespaceService)
	usecase := setupPrivateUsecase(mockService, domain.Quotas{})

	mockService.On(
		"CreateNamespace",
		mock.Anything, userNamespace, mock.Anything, mock.Anything,
	).Return(ports.NamespaceCreationResult(""), errors.New("failed to create namespace"))

	err := usecase.createNamespace(context.Background(), userNamespace)

	assert.Error(t, err)
	mockService.AssertCalled(
		t,
		"CreateNamespace",
		mock.Anything,
		userNamespace,
		mock.Anything,
		mock.Anything,
	)
}

func TestGetNamespaceAnnotationsDisabled(t *testing.T) {
	usecase := setupPrivateUsecase(new(MockNamespaceService), domain.Quotas{})
	usecase.namespace.Annotation.Enabled = false

	annotations := usecase.getNamespaceAnnotations(context.Background())

	assert.Nil(t, annotations, "Expected nil when annotations are disabled")
}

func TestGetNamespaceAnnotationsStaticOnly(t *testing.T) {
	usecase := setupPrivateUsecase(new(MockNamespaceService), domain.Quotas{})
	usecase.namespace.Annotation.Enabled = true
	usecase.namespace.Annotation.Static = map[string]string{
		"static-key": "static-value",
	}

	annotations := usecase.getNamespaceAnnotations(context.Background())

	assert.NotNil(t, annotations)
	assert.Equal(t, "static-value", annotations["static-key"])
}

func TestGetNamespaceAnnotationsLastLoginTimestamp(t *testing.T) {
	usecase := setupPrivateUsecase(new(MockNamespaceService), domain.Quotas{})
	usecase.namespace.Annotation.Enabled = true
	usecase.namespace.Annotation.Dynamic.LastLoginTimestamp = true

	before := time.Now().Add(-2 * time.Second).UnixMilli()
	annotations := usecase.getNamespaceAnnotations(context.Background())
	after := time.Now().Add(+2 * time.Second).UnixMilli()

	v := annotations["onyxia_last_login_timestamp"]
	ms, err := strconv.ParseInt(v, 10, 64)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, ms, before)
	assert.LessOrEqual(t, ms, after)
}

func TestGetNamespaceAnnotationsUserAttributes(t *testing.T) {

	ctx, reader, _ := usercontext.NewTestUserContext(&usercontext.User{
		Attributes: map[string]any{
			"user-attr1": "value1",
			"user-attr2": "value2",
		},
	})

	usecase := setupPrivateUsecase(new(MockNamespaceService), domain.Quotas{})
	usecase.namespace.Annotation.Enabled = true
	usecase.namespace.Annotation.Dynamic.UserAttributes = []string{"user-attr1", "user-attr2"}
	usecase.userContextReader = reader

	annotations := usecase.getNamespaceAnnotations(ctx)

	assert.NotNil(t, annotations)
	assert.Equal(t, "value1", annotations["user-attr1"])
	assert.Equal(t, "value2", annotations["user-attr2"])
}

func TestGetNamespaceAnnotationsAllAnnotations(t *testing.T) {

	ctx, reader, _ := usercontext.NewTestUserContext(&usercontext.User{
		Attributes: map[string]any{
			"user-attr1": "value1",
		},
	})

	usecase := setupPrivateUsecase(new(MockNamespaceService), domain.Quotas{})
	usecase.namespace.Annotation.Enabled = true
	usecase.namespace.Annotation.Static = map[string]string{
		"static-key": "static-value",
	}
	usecase.namespace.Annotation.Dynamic.LastLoginTimestamp = true
	usecase.namespace.Annotation.Dynamic.UserAttributes = []string{"user-attr1"}
	usecase.userContextReader = reader

	annotations := usecase.getNamespaceAnnotations(ctx)

	assert.NotNil(t, annotations)
	assert.Equal(t, "static-value", annotations["static-key"])
	assert.Contains(t, annotations, "onyxia_last_login_timestamp")
	assert.Equal(t, "value1", annotations["user-attr1"])
}

// The static annotations are shared configuration: building a namespace's
// annotations must not write into them (concurrent requests would race, and
// one user's dynamic annotations would leak into the next request).
func TestGetNamespaceAnnotationsDoesNotMutateStaticConfig(t *testing.T) {
	usecase := setupPrivateUsecase(new(MockNamespaceService), domain.Quotas{})
	usecase.namespace.Annotation.Enabled = true
	usecase.namespace.Annotation.Static = map[string]string{"static-key": "static-value"}
	usecase.namespace.Annotation.Dynamic.LastLoginTimestamp = true

	annotations := usecase.getNamespaceAnnotations(context.Background())

	assert.Contains(t, annotations, "onyxia_last_login_timestamp")
	assert.Equal(t, map[string]string{"static-key": "static-value"}, usecase.namespace.Annotation.Static)
}
