package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestApplyQuotasSuccess(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{
		Enabled: true,
		Default: domain.Quota{MemoryRequest: "10Gi"},
	}
	usecase := setupPrivateUsecase(mockService, quotas)

	mockService.On("ApplyResourceQuotas", mock.Anything, userNamespace, &quotas.Default).
		Return(port.QuotaCreated, nil)

	err := usecase.applyQuotas(
		context.Background(),
		userNamespace,
		domain.OnboardingRequest{UserName: testUserName},
	)

	assert.NoError(t, err)
	mockService.AssertCalled(
		t,
		"ApplyResourceQuotas",
		mock.Anything,
		userNamespace,
		&quotas.Default,
	)
}

func TestApplyQuotasAlreadyUpToDate(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{
		Enabled: true,
		Default: domain.Quota{MemoryRequest: "10Gi"},
	}
	usecase := setupPrivateUsecase(mockService, quotas)

	mockService.On("ApplyResourceQuotas", mock.Anything, userNamespace, &quotas.Default).
		Return(port.QuotaUnchanged, nil)

	err := usecase.applyQuotas(
		context.Background(),
		userNamespace,
		domain.OnboardingRequest{UserName: testUserName},
	)

	assert.NoError(t, err)
	mockService.AssertCalled(
		t,
		"ApplyResourceQuotas",
		mock.Anything,
		userNamespace,
		&quotas.Default,
	)
}

func TestApplyQuotasQuotasDisabled(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{Enabled: false}
	usecase := setupPrivateUsecase(mockService, quotas)

	err := usecase.applyQuotas(
		context.Background(),
		userNamespace,
		domain.OnboardingRequest{UserName: testUserName},
	)

	assert.NoError(t, err)
	mockService.AssertNotCalled(t, "ApplyResourceQuotas")
}

func TestApplyQuotasQuotaUpdated(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{
		Enabled: true,
		Default: domain.Quota{MemoryRequest: "10Gi"},
	}
	usecase := setupPrivateUsecase(mockService, quotas)

	mockService.On("ApplyResourceQuotas", mock.Anything, userNamespace, &quotas.Default).
		Return(port.QuotaUpdated, nil)

	err := usecase.applyQuotas(
		context.Background(),
		userNamespace,
		domain.OnboardingRequest{UserName: testUserName},
	)

	assert.NoError(t, err)
	mockService.AssertCalled(
		t,
		"ApplyResourceQuotas",
		mock.Anything,
		userNamespace,
		&quotas.Default,
	)
}

func TestApplyQuotasQuotaIgnored(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{
		Enabled: true,
		Default: domain.Quota{MemoryRequest: "10Gi"},
	}
	usecase := setupPrivateUsecase(mockService, quotas)

	mockService.On("ApplyResourceQuotas", mock.Anything, userNamespace, &quotas.Default).
		Return(port.QuotaIgnored, nil)

	err := usecase.applyQuotas(
		context.Background(),
		userNamespace,
		domain.OnboardingRequest{UserName: testUserName},
	)

	assert.NoError(t, err)
	mockService.AssertCalled(
		t,
		"ApplyResourceQuotas",
		mock.Anything,
		userNamespace,
		&quotas.Default,
	)
}
func TestApplyQuotasFailure(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{
		Enabled: true,
		Default: domain.Quota{MemoryRequest: "10Gi"},
	}
	usecase := setupPrivateUsecase(mockService, quotas)

	mockService.On("ApplyResourceQuotas", mock.Anything, userNamespace, &quotas.Default).
		Return(port.QuotaApplicationResult(""), errors.New("failed to apply quotas"))
	err := usecase.applyQuotas(
		context.Background(),
		userNamespace,
		domain.OnboardingRequest{UserName: testUserName},
	)

	assert.Error(t, err)
	mockService.AssertCalled(
		t,
		"ApplyResourceQuotas",
		mock.Anything,
		userNamespace,
		&quotas.Default,
	)
}

func TestGetQuotaGroupQuota(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{
		Enabled:      true,
		GroupEnabled: true,
		Group:        domain.Quota{MemoryRequest: "12Gi"},
	}
	usecase := setupPrivateUsecase(mockService, quotas)

	groupName := testGroupName
	req := domain.OnboardingRequest{Group: &groupName, UserName: testUserName}

	quota := usecase.getQuota(context.Background(), req, groupNamespace)

	assert.Equal(t, &quotas.Group, quota)
}

func TestGetGroupQuotaFallbackToDefault(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{
		Enabled:      true,
		GroupEnabled: false,                               // ❌ Group quotas disabled
		Default:      domain.Quota{MemoryRequest: "10Gi"}, // ✅ Default quota exists
		Group:        domain.Quota{MemoryRequest: "20Gi"}, // 🚨 Should not be used
	}
	usecase := setupPrivateUsecase(mockService, quotas)

	groupName := testGroupName
	req := domain.OnboardingRequest{UserName: testUserName, Group: &groupName}

	quota := usecase.getGroupQuota(context.Background(), req, userNamespace)

	// ✅ Expected: Fallback to `quotas.Default`
	assert.Equal(
		t,
		&quotas.Default,
		quota,
		"Expected fallback to default quota when group quotas are disabled",
	)
}

func TestGetQuotaUserQuota(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{
		Enabled:     true,
		UserEnabled: true,
		User:        domain.Quota{MemoryRequest: "11Gi"},
	}
	usecase := setupPrivateUsecase(mockService, quotas)

	req := domain.OnboardingRequest{Group: nil, UserName: testUserName}

	quota := usecase.getQuota(context.Background(), req, userNamespace)

	assert.Equal(t, &quotas.User, quota)
}

func TestGetQuotaDefaultQuota(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{
		Enabled: true,
		Default: domain.Quota{MemoryRequest: "10Gi"},
	}
	usecase := setupPrivateUsecase(mockService, quotas)

	req := domain.OnboardingRequest{Group: nil, UserName: testUserName}

	quota := usecase.getQuota(context.Background(), req, userNamespace)

	assert.Equal(t, &quotas.Default, quota)
}

func TestGetQuotaRoleQuota(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{
		Enabled: true,
		Roles: map[string]domain.Quota{
			"admin": {MemoryRequest: "16Gi"},
		},
	}
	usecase := setupPrivateUsecase(mockService, quotas)

	req := domain.OnboardingRequest{
		UserName:  testUserName,
		UserRoles: []string{"admin"}, // ✅ Only one role, should be used
	}

	quota := usecase.getQuota(context.Background(), req, userNamespace)

	expectedQuota := quotas.Roles["admin"]
	assert.Equal(t, &expectedQuota, quota, "Expected 'admin' role quota")
}

func TestGetQuotaRoleQuotaAppliesFirstMatch(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{
		Enabled: true,
		Roles: map[string]domain.Quota{
			"admin":     {MemoryRequest: "16Gi"},
			"developer": {MemoryRequest: "14Gi"},
		},
	}
	usecase := setupPrivateUsecase(mockService, quotas)

	req := domain.OnboardingRequest{
		UserName:  testUserName,
		UserRoles: []string{"developer", "admin"}, // ✅ "developer" should be used
	}

	quota := usecase.getQuota(context.Background(), req, userNamespace)

	expectedQuota := quotas.Roles["developer"] // ✅ Copy value before taking address
	assert.Equal(t, &expectedQuota, quota, "Expected the first matching role's quota")
}

func TestGetQuotaUserQuotaWhenNoRoleMatches(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{
		Enabled:     true,
		UserEnabled: true,
		User:        domain.Quota{MemoryRequest: "12Gi"},
		Roles: map[string]domain.Quota{
			"admin":     {MemoryRequest: "16Gi"},
			"developer": {MemoryRequest: "14Gi"},
		},
	}
	usecase := setupPrivateUsecase(mockService, quotas)

	req := domain.OnboardingRequest{
		UserName:  testUserName,
		UserRoles: []string{"nonexistent-role"}, // ❌ Role is not in the quota map
	}

	quota := usecase.getQuota(context.Background(), req, userNamespace)

	expectedQuota := quotas.User
	assert.Equal(t, &expectedQuota, quota, "Expected fallback to user quota when no role matches")
}

func TestGetQuotaDefaultQuotaWhenNoRoleAndUserQuotaDisabled(t *testing.T) {
	mockService := new(MockNamespaceService)
	quotas := domain.Quotas{
		Enabled: true,
		Default: domain.Quota{MemoryRequest: "10Gi"},
		User:    domain.Quota{MemoryRequest: "12Gi"},
	}
	usecase := setupPrivateUsecase(mockService, quotas)

	req := domain.OnboardingRequest{
		UserName:  testUserName,
		UserRoles: []string{}, // ✅ No roles provided
	}

	quota := usecase.getQuota(context.Background(), req, userNamespace)

	expectedQuota := quotas.Default
	assert.Equal(t, &expectedQuota, quota, "Expected default quota when no role/user quota applies")
}
