package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	api "github.com/onyxia-datalab/onyxia-backend/onboarding/api/oas"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/ports"
)

// ✅ Mock `OnboardingUsecase`
type MockOnboardingUsecase struct {
	mock.Mock
}

var _ ports.OnboardingUsecase = (*MockOnboardingUsecase)(nil)

func (m *MockOnboardingUsecase) Onboard(ctx context.Context, req domain.OnboardingRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func TestOnboardSuccessNoGroup(t *testing.T) {
	mockUC := new(MockOnboardingUsecase)
	ctx, userCtxReader, _ := usercontext.NewTestUserContext(&usercontext.User{
		Username: "test-user",
		Groups:   []string{},
		Roles:    []string{"test-role"},
	})

	mockUC.On("Onboard", mock.Anything, mock.Anything).Return(nil)

	ctrl := NewOnboardingController(mockUC, userCtxReader)
	req := api.OnboardingRequest{Group: api.OptString{Set: false}}

	res, err := ctrl.Onboard(ctx, &req)
	assert.NoError(t, err)
	assert.IsType(t, &api.OnboardOK{}, res)
}

func TestOnboardGetUserFails(t *testing.T) {
	mockUC := new(MockOnboardingUsecase)
	ctx, userCtxReader, _ := usercontext.NewTestUserContext(nil)

	ctrl := NewOnboardingController(mockUC, userCtxReader)
	req := api.OnboardingRequest{Group: api.OptString{Value: "g", Set: true}}

	res, err := ctrl.Onboard(ctx, &req)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrForbidden)
	mockUC.AssertNotCalled(t, "Onboard")
}

func TestOnboardGroupValidationFails(t *testing.T) {
	mockUC := new(MockOnboardingUsecase)
	ctx, userCtxReader, _ := usercontext.NewTestUserContext(&usercontext.User{
		Username: "u",
		Groups:   []string{"not-test-group"},
		Roles:    []string{"r"},
	})

	ctrl := NewOnboardingController(mockUC, userCtxReader)
	req := api.OnboardingRequest{Group: api.OptString{Value: "test-group", Set: true}}

	res, err := ctrl.Onboard(ctx, &req)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrForbidden)
	mockUC.AssertNotCalled(t, "Onboard")
}

// A plain usecase failure isn't one of the shared sentinels: it should pass
// through untouched and default to 500 in the central error handler, not be
// folded into a 4xx the way it silently was before (the old typed response
// was never actually reachable — ogen discards it whenever the handler also
// returns a non-nil error, which every branch here did).
func TestOnboardOnboardingFails(t *testing.T) {
	mockUC := new(MockOnboardingUsecase)
	ctx, userCtxReader, _ := usercontext.NewTestUserContext(&usercontext.User{
		Username: "u",
		Groups:   []string{"test-group"},
		Roles:    []string{"r"},
	})

	usecaseErr := errors.New("boom")
	mockUC.On("Onboard", mock.Anything, mock.Anything).Return(usecaseErr)

	ctrl := NewOnboardingController(mockUC, userCtxReader)
	req := api.OnboardingRequest{Group: api.OptString{Value: "test-group", Set: true}}

	res, err := ctrl.Onboard(ctx, &req)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, usecaseErr)
	mockUC.AssertCalled(t, "Onboard", mock.Anything, mock.Anything)
}
