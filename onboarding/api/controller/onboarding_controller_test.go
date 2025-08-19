package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	api "github.com/onyxia-datalab/onyxia-backend/onboarding/api/oas"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ✅ Mock `OnboardingUsecase`
type MockOnboardingUsecase struct {
	mock.Mock
}

var _ domain.OnboardingUsecase = (*MockOnboardingUsecase)(nil)

func (m *MockOnboardingUsecase) Onboard(ctx context.Context, req domain.OnboardingRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func TestOnboard_Success_NoGroup(t *testing.T) {
	mockUC := new(MockOnboardingUsecase)
	getUser := func(ctx context.Context) (*usercontext.User, bool) {
		return &usercontext.User{
			Username: "test-user",
			Groups:   []string{"g1", "g2"},
			Roles:    []string{"r1"},
		}, true
	}
	mockUC.On("Onboard", mock.Anything, mock.Anything).Return(nil)

	ctrl := NewOnboardingController(mockUC, getUser)
	req := api.OnboardingRequest{Group: api.OptString{Set: false}}

	res, err := ctrl.Onboard(context.Background(), &req)
	assert.NoError(t, err)
	assert.IsType(t, &api.OnboardOK{}, res)
}

func TestOnboard_GetUserFails(t *testing.T) {
	mockUC := new(MockOnboardingUsecase)
	getUser := func(ctx context.Context) (*usercontext.User, bool) { return nil, false }

	ctrl := NewOnboardingController(mockUC, getUser)
	req := api.OnboardingRequest{Group: api.OptString{Value: "g", Set: true}}

	res, err := ctrl.Onboard(context.Background(), &req)
	assert.Error(t, err)
	assert.IsType(t, &api.OnboardForbidden{}, res)
	mockUC.AssertNotCalled(t, "Onboard")
}

func TestOnboard_GroupValidationFails(t *testing.T) {
	mockUC := new(MockOnboardingUsecase)
	getUser := func(ctx context.Context) (*usercontext.User, bool) {
		return &usercontext.User{
			Username: "u",
			Groups:   []string{"other"},
			Roles:    []string{"r"},
		}, true
	}

	ctrl := NewOnboardingController(mockUC, getUser)
	req := api.OnboardingRequest{Group: api.OptString{Value: "test-group", Set: true}}

	res, err := ctrl.Onboard(context.Background(), &req)
	assert.Error(t, err)
	assert.IsType(t, &api.OnboardUnauthorized{}, res)
	mockUC.AssertNotCalled(t, "Onboard")
}

func TestOnboard_OnboardingFails(t *testing.T) {
	mockUC := new(MockOnboardingUsecase)
	getUser := func(ctx context.Context) (*usercontext.User, bool) {
		return &usercontext.User{
			Username: "u",
			Groups:   []string{"test-group"},
			Roles:    []string{"r"},
		}, true
	}
	mockUC.On("Onboard", mock.Anything, mock.Anything).Return(errors.New("boom"))

	ctrl := NewOnboardingController(mockUC, getUser)
	req := api.OnboardingRequest{Group: api.OptString{Value: "test-group", Set: true}}

	res, err := ctrl.Onboard(context.Background(), &req)
	assert.Error(t, err)
	assert.IsType(t, &api.OnboardForbidden{}, res)
	mockUC.AssertCalled(t, "Onboard", mock.Anything, mock.Anything)
}
