package controller

import (
	"context"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	api "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type lifecycleStub struct {
	start      func(context.Context, domain.StartRequest) error
	suspendErr error
	resumeErr  error
	deleteErr  error
	sharedReq  *domain.SetSharedRequest
	sharedErr  error
}

func (s *lifecycleStub) Start(
	ctx context.Context,
	req domain.StartRequest,
) error {
	if s.start == nil {
		return nil
	}
	return s.start(ctx, req)
}

func (s *lifecycleStub) Suspend(context.Context, domain.SuspendRequest) error {
	return s.suspendErr
}

func (s *lifecycleStub) Resume(context.Context, domain.ResumeRequest) error {
	return s.resumeErr
}

func (s *lifecycleStub) Delete(context.Context, domain.DeleteRequest) error {
	return s.deleteErr
}

func (s *lifecycleStub) SetShared(_ context.Context, req domain.SetSharedRequest) error {
	s.sharedReq = &req
	return s.sharedErr
}

func TestInstallServiceSelectsPackageVersionAndCanonicalReleaseID(t *testing.T) {
	tests := []struct {
		name           string
		packageVersion api.OptString
		wantVersion    string
	}{
		{
			name:           "packageVersion is passed on",
			packageVersion: api.NewOptString("2.0.0"),
			wantVersion:    "2.0.0",
		},
		{
			name:        "empty version selects latest",
			wantVersion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, users, _ := usercontext.NewTestUserContext(&usercontext.User{Username: "alice"})
			var captured domain.StartRequest
			lifecycle := &lifecycleStub{start: func(
				_ context.Context,
				req domain.StartRequest,
			) error {
				captured = req
				return nil
			}}
			ctrl := NewInstallController(lifecycle, users)

			res, err := ctrl.InstallService(ctx, &api.ServiceInstallRequest{
				CatalogId:      "catalog",
				PackageName:    "jupyter",
				PackageVersion: tt.packageVersion,
				Options:        api.ServiceInstallRequestOptions{},
			}, api.InstallServiceParams{
				ReleaseId:      "release-id",
				XOnyxiaProject: "user-alice",
			})

			require.NoError(t, err)
			assert.IsType(t, &api.InstallAcceptedHeaders{}, res)
			assert.Equal(t, tt.wantVersion, captured.Version)
			assert.Equal(t, "release-id", captured.ReleaseID)
			assert.Equal(t, "user-alice", captured.Namespace)
		})
	}
}

// The controller passes the whole caller on: the use case decides, from the
// username and the groups, whether the namespace is theirs.
func TestSetServiceSharedPassesCallerAndFlag(t *testing.T) {
	caller := usercontext.User{Username: "alice", Groups: []string{"data-team"}}
	ctx, users, _ := usercontext.NewTestUserContext(&caller)
	lifecycle := &lifecycleStub{}
	ctrl := NewInstallController(lifecycle, users)

	res, err := ctrl.SetServiceShared(ctx, &api.SetServiceSharedReq{Shared: true}, api.SetServiceSharedParams{
		ReleaseId:      "release-id",
		XOnyxiaProject: "user-alice",
	})

	require.NoError(t, err)
	assert.IsType(t, &api.SetServiceSharedNoContent{}, res)
	require.NotNil(t, lifecycle.sharedReq)
	assert.Equal(t, domain.SetSharedRequest{
		User:        caller,
		ReleaseName: "release-id",
		Namespace:   "user-alice",
		Shared:      true,
	}, *lifecycle.sharedReq)
}

func TestSetServiceSharedWithoutUserIsForbidden(t *testing.T) {
	users, _ := usercontext.NewUserContext()
	ctrl := NewInstallController(&lifecycleStub{}, users)

	res, err := ctrl.SetServiceShared(context.Background(), &api.SetServiceSharedReq{}, api.SetServiceSharedParams{
		ReleaseId:      "release-id",
		XOnyxiaProject: "user-alice",
	})

	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestSetServiceSharedPropagatesUsecaseError(t *testing.T) {
	ctx, users, _ := usercontext.NewTestUserContext(&usercontext.User{Username: "alice"})
	ctrl := NewInstallController(&lifecycleStub{sharedErr: domain.ErrForbidden}, users)

	res, err := ctrl.SetServiceShared(ctx, &api.SetServiceSharedReq{Shared: true}, api.SetServiceSharedParams{
		ReleaseId:      "release-id",
		XOnyxiaProject: "user-alice",
	})

	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

// A missing (or hidden) catalog or package is a 404, as the spec declares.
func TestInstallServicePropagatesNotFound(t *testing.T) {
	ctx, users, _ := usercontext.NewTestUserContext(&usercontext.User{Username: "alice"})
	lifecycle := &lifecycleStub{start: func(
		context.Context,
		domain.StartRequest,
	) error {
		return domain.ErrNotFound
	}}
	ctrl := NewInstallController(lifecycle, users)

	res, err := ctrl.InstallService(ctx, &api.ServiceInstallRequest{
		CatalogId:   "catalog",
		PackageName: "jupyter",
		Options:     api.ServiceInstallRequestOptions{},
	}, api.InstallServiceParams{
		ReleaseId:      "release-id",
		XOnyxiaProject: "user-alice",
	})

	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.NotErrorIs(t, err, domain.ErrInvalidInput)
}
