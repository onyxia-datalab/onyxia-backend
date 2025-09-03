package usecase

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateNamespace_Success(t *testing.T) {
	mockService := new(MockNamespaceService)
	usecase := setupPrivateUsecase(mockService, domain.Quotas{})

	mockService.On(
		"CreateNamespace",
		mock.Anything, // ctx
		userNamespace, // name
		mock.Anything, // annotations
		mock.Anything, // labels
	).Return(port.NamespaceCreated, nil)

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

func TestCreateNamespace_AlreadyExists(t *testing.T) {
	mockService := new(MockNamespaceService)
	usecase := setupPrivateUsecase(mockService, domain.Quotas{})

	mockService.On(
		"CreateNamespace",
		mock.Anything, userNamespace, mock.Anything, mock.Anything,
	).Return(port.NamespaceAlreadyExists, nil)

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

func TestCreateNamespace_Failure(t *testing.T) {
	mockService := new(MockNamespaceService)
	usecase := setupPrivateUsecase(mockService, domain.Quotas{})

	mockService.On(
		"CreateNamespace",
		mock.Anything, userNamespace, mock.Anything, mock.Anything,
	).Return(port.NamespaceCreationResult(""), errors.New("failed to create namespace"))

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

func TestGetNamespaceAnnotations_Disabled(t *testing.T) {
	usecase := setupPrivateUsecase(new(MockNamespaceService), domain.Quotas{})
	usecase.namespace.Annotation.Enabled = false

	annotations := usecase.getNamespaceAnnotations(context.Background())

	assert.Nil(t, annotations, "Expected nil when annotations are disabled")
}

func TestGetNamespaceAnnotations_StaticOnly(t *testing.T) {
	usecase := setupPrivateUsecase(new(MockNamespaceService), domain.Quotas{})
	usecase.namespace.Annotation.Enabled = true
	usecase.namespace.Annotation.Static = map[string]string{
		"static-key": "static-value",
	}

	annotations := usecase.getNamespaceAnnotations(context.Background())

	assert.NotNil(t, annotations)
	assert.Equal(t, "static-value", annotations["static-key"])
}

func TestGetNamespaceAnnotations_LastLoginTimestamp(t *testing.T) {
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

func TestGetNamespaceAnnotations_UserAttributes(t *testing.T) {

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

func TestGetNamespaceAnnotations_AllAnnotations(t *testing.T) {

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
