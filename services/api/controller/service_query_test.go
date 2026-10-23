package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	api "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type serviceQueryStub struct {
	svc     domain.Service
	svcs    []domain.Service
	err     error
	gotUser usercontext.User
	gotNs   string
	gotID   string
}

func (s *serviceQueryStub) GetService(
	_ context.Context,
	user usercontext.User,
	namespace, releaseID string,
) (domain.Service, error) {
	s.gotUser, s.gotNs, s.gotID = user, namespace, releaseID
	return s.svc, s.err
}

func (s *serviceQueryStub) ListServices(
	_ context.Context,
	user usercontext.User,
	namespace string,
) ([]domain.Service, error) {
	s.gotUser, s.gotNs = user, namespace
	return s.svcs, s.err
}

func TestGetServiceMapsService(t *testing.T) {
	caller := usercontext.User{Username: "alice"}
	stub := &serviceQueryStub{svc: domain.Service{
		ReleaseID:    "jupyter-1",
		FriendlyName: "My notebook",
		Owner:        "alice",
		CatalogID:    "ide",
		Share:        true,
		Status:       domain.ServiceStatusError,
		Error: &domain.ServiceError{
			Reason:       domain.ServiceErrorReasonOOMKilled,
			PodName:      "jupyter-1-0",
			Message:      "killed",
			RestartCount: 3,
			ExitCode:     137,
			Image:        "jupyter:latest",
			Limit:        "2Gi",
		},
	}}
	ctx, users, _ := usercontext.NewTestUserContext(&caller)
	ctrl := NewServiceQueryController(stub, users)

	res, err := ctrl.GetService(ctx, api.GetServiceParams{XOnyxiaProject: "user-alice", ReleaseId: "jupyter-1"})

	require.NoError(t, err)
	assert.Equal(t, caller, stub.gotUser)
	assert.Equal(t, "user-alice", stub.gotNs)
	assert.Equal(t, "jupyter-1", stub.gotID)
	assert.Equal(t, &api.Service{
		ReleaseId:    "jupyter-1",
		Status:       api.ServiceStatusError,
		FriendlyName: "My notebook",
		Owner:        "alice",
		CatalogId:    "ide",
		Share:        true,
		Error: api.NewOptServiceError(api.ServiceError{
			Reason:       api.ServiceErrorReasonOomKilled,
			PodName:      api.NewOptString("jupyter-1-0"),
			Message:      api.NewOptString("killed"),
			RestartCount: api.NewOptInt(3),
			ExitCode:     api.NewOptInt(137),
			Image:        api.NewOptString("jupyter:latest"),
			Limit:        api.NewOptString("2Gi"),
		}),
	}, res)
}

// A release failure has no pod behind it: the pod name is left out.
func TestGetServiceReleaseFailedHasNoPodName(t *testing.T) {
	stub := &serviceQueryStub{svc: domain.Service{
		Status: domain.ServiceStatusError,
		Error:  &domain.ServiceError{Reason: domain.ServiceErrorReasonReleaseFailed},
	}}
	ctx, users, _ := usercontext.NewTestUserContext(&usercontext.User{Username: "alice"})
	ctrl := NewServiceQueryController(stub, users)

	res, err := ctrl.GetService(ctx, api.GetServiceParams{})

	require.NoError(t, err)
	svcErr, ok := res.(*api.Service).Error.Get()
	require.True(t, ok)
	assert.Equal(t, api.ServiceErrorReasonReleaseFailed, svcErr.Reason)
	assert.False(t, svcErr.PodName.IsSet())
}

func TestGetServiceErrors(t *testing.T) {
	boom := errors.New("boom")
	for name, want := range map[string]error{"not found": domain.ErrNotFound, "failure": boom} {
		t.Run(name, func(t *testing.T) {
			ctx, users, _ := usercontext.NewTestUserContext(&usercontext.User{Username: "alice"})
			ctrl := NewServiceQueryController(&serviceQueryStub{err: want}, users)

			res, err := ctrl.GetService(ctx, api.GetServiceParams{})

			assert.Nil(t, res)
			assert.ErrorIs(t, err, want)
		})
	}
}

func TestGetServiceWithoutUserIsForbidden(t *testing.T) {
	users, _ := usercontext.NewUserContext()
	ctrl := NewServiceQueryController(&serviceQueryStub{}, users)

	res, err := ctrl.GetService(context.Background(), api.GetServiceParams{})

	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestListServices(t *testing.T) {
	stub := &serviceQueryStub{svcs: []domain.Service{
		{ReleaseID: "a", Status: domain.ServiceStatusRunning},
		{ReleaseID: "b", Status: domain.ServiceStatusSuspended},
	}}
	ctx, users, _ := usercontext.NewTestUserContext(&usercontext.User{Username: "alice"})
	ctrl := NewServiceQueryController(stub, users)

	res, err := ctrl.ListServices(ctx, api.ListServicesParams{XOnyxiaProject: "user-alice"})

	require.NoError(t, err)
	assert.Equal(t, "user-alice", stub.gotNs)
	svcs := *res.(*api.ListServicesOKApplicationJSON)
	require.Len(t, svcs, 2)
	assert.Equal(t, "a", svcs[0].ReleaseId)
	assert.Equal(t, api.ServiceStatusRunning, svcs[0].Status)
	assert.False(t, svcs[0].Error.IsSet())
	assert.Equal(t, api.ServiceStatusSuspended, svcs[1].Status)
}

func TestListServicesErrors(t *testing.T) {
	boom := errors.New("boom")
	ctx, users, _ := usercontext.NewTestUserContext(&usercontext.User{Username: "alice"})
	ctrl := NewServiceQueryController(&serviceQueryStub{err: boom}, users)

	res, err := ctrl.ListServices(ctx, api.ListServicesParams{})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, boom)

	anonymous, _ := usercontext.NewUserContext()
	res, err = NewServiceQueryController(&serviceQueryStub{}, anonymous).
		ListServices(context.Background(), api.ListServicesParams{})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestToAPIStatus(t *testing.T) {
	for in, want := range map[domain.ServiceStatus]api.ServiceStatus{
		domain.ServiceStatusDeploying:   api.ServiceStatusDeploying,
		domain.ServiceStatusRunning:     api.ServiceStatusRunning,
		domain.ServiceStatusError:       api.ServiceStatusError,
		domain.ServiceStatusGhost:       api.ServiceStatusGhost,
		domain.ServiceStatusSuspending:  api.ServiceStatusSuspending,
		domain.ServiceStatusSuspended:   api.ServiceStatusSuspended,
		domain.ServiceStatusTerminating: api.ServiceStatusTerminating,
		"unknown":                       api.ServiceStatusDeploying,
	} {
		assert.Equal(t, want, toAPIStatus(in), in)
	}
}

func TestToAPIErrorReason(t *testing.T) {
	for in, want := range map[domain.ServiceErrorReason]api.ServiceErrorReason{
		domain.ServiceErrorReasonReleaseFailed:   api.ServiceErrorReasonReleaseFailed,
		domain.ServiceErrorReasonCrashLoop:       api.ServiceErrorReasonCrashLoop,
		domain.ServiceErrorReasonOOMKilled:       api.ServiceErrorReasonOomKilled,
		domain.ServiceErrorReasonImagePull:       api.ServiceErrorReasonImagePull,
		domain.ServiceErrorReasonConfigError:     api.ServiceErrorReasonConfigError,
		domain.ServiceErrorReasonUnschedulable:   api.ServiceErrorReasonUnschedulable,
		domain.ServiceErrorReasonReadinessFailed: api.ServiceErrorReasonReadinessFailed,
		"unknown":                                api.ServiceErrorReasonCrashLoop,
	} {
		assert.Equal(t, want, toAPIErrorReason(in), in)
	}
}
