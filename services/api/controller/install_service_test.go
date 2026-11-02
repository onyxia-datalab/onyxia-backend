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
			require.IsType(t, &api.InstallServiceAccepted{}, res)
			assert.Equal(t, api.NewOptString("/api/services/release-id"), res.(*api.InstallServiceAccepted).Location)
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

func TestInstallServiceBuildsStartRequest(t *testing.T) {
	caller := usercontext.User{Username: "alice"}
	ctx, users, _ := usercontext.NewTestUserContext(&caller)
	var captured domain.StartRequest
	ctrl := NewInstallController(&lifecycleStub{start: func(_ context.Context, req domain.StartRequest) error {
		captured = req
		return nil
	}}, users)

	_, err := ctrl.InstallService(ctx, &api.ServiceInstallRequest{
		CatalogId:    "catalog",
		PackageName:  "jupyter",
		FriendlyName: api.NewOptString("My notebook"),
		Share:        api.NewOptBool(true),
		Options: api.ServiceInstallRequestOptions{
			"replicas":  []byte(`2`),
			"resources": []byte(`{"cpu":"1"}`),
		},
	}, api.InstallServiceParams{ReleaseId: "release-id", XOnyxiaProject: "user-alice"})

	require.NoError(t, err)
	assert.Equal(t, caller, captured.User)
	assert.Equal(t, "catalog", captured.CatalogID)
	assert.Equal(t, "jupyter", captured.PackageName)
	assert.Equal(t, "My notebook", captured.FriendlyName)
	assert.True(t, captured.Share)
	assert.Equal(t, map[string]interface{}{
		"replicas":  float64(2),
		"resources": map[string]interface{}{"cpu": "1"},
	}, captured.Values)
}

func TestInstallServiceDefaultsFriendlyNameToPackageName(t *testing.T) {
	ctx, users, _ := usercontext.NewTestUserContext(&usercontext.User{Username: "alice"})
	var captured domain.StartRequest
	ctrl := NewInstallController(&lifecycleStub{start: func(_ context.Context, req domain.StartRequest) error {
		captured = req
		return nil
	}}, users)

	_, err := ctrl.InstallService(ctx, &api.ServiceInstallRequest{
		CatalogId:   "catalog",
		PackageName: "jupyter",
		Options:     api.ServiceInstallRequestOptions{},
	}, api.InstallServiceParams{})

	require.NoError(t, err)
	assert.Equal(t, "jupyter", captured.FriendlyName)
	assert.False(t, captured.Share)
}

func TestInstallServiceRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		req  *api.ServiceInstallRequest
	}{
		{name: "no body"},
		{name: "no package", req: &api.ServiceInstallRequest{CatalogId: "catalog", Options: api.ServiceInstallRequestOptions{}}},
		{name: "no catalog", req: &api.ServiceInstallRequest{PackageName: "jupyter", Options: api.ServiceInstallRequestOptions{}}},
		{name: "no options", req: &api.ServiceInstallRequest{CatalogId: "catalog", PackageName: "jupyter"}},
		{name: "malformed option", req: &api.ServiceInstallRequest{
			CatalogId:   "catalog",
			PackageName: "jupyter",
			Options:     api.ServiceInstallRequestOptions{"bad": []byte(`{`)},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, users, _ := usercontext.NewTestUserContext(&usercontext.User{Username: "alice"})
			started := false
			ctrl := NewInstallController(&lifecycleStub{start: func(context.Context, domain.StartRequest) error {
				started = true
				return nil
			}}, users)

			res, err := ctrl.InstallService(ctx, tt.req, api.InstallServiceParams{})

			assert.Nil(t, res)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.False(t, started)
		})
	}
}

func TestInstallServiceWithoutUserIsForbidden(t *testing.T) {
	users, _ := usercontext.NewUserContext()
	ctrl := NewInstallController(&lifecycleStub{}, users)

	res, err := ctrl.InstallService(context.Background(), &api.ServiceInstallRequest{}, api.InstallServiceParams{})

	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestSetServiceSuspended(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name      string
		suspended bool
		stub      *lifecycleStub
		wantErr   error
	}{
		{name: "suspend", suspended: true, stub: &lifecycleStub{}},
		{name: "resume", suspended: false, stub: &lifecycleStub{}},
		{name: "suspend failure", suspended: true, stub: &lifecycleStub{suspendErr: boom}, wantErr: boom},
		{name: "resume failure", suspended: false, stub: &lifecycleStub{resumeErr: boom}, wantErr: boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, users, _ := usercontext.NewTestUserContext(&usercontext.User{Username: "alice"})
			ctrl := NewInstallController(tt.stub, users)

			res, err := ctrl.SetServiceSuspended(
				ctx,
				&api.SetServiceSuspendedReq{Suspended: tt.suspended},
				api.SetServiceSuspendedParams{ReleaseId: "release-id", XOnyxiaProject: "user-alice"},
			)

			if tt.wantErr != nil {
				assert.Nil(t, res)
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.IsType(t, &api.SetServiceSuspendedNoContent{}, res)
		})
	}
}

func TestSetServiceSuspendedWithoutUserIsForbidden(t *testing.T) {
	users, _ := usercontext.NewUserContext()
	ctrl := NewInstallController(&lifecycleStub{}, users)

	res, err := ctrl.SetServiceSuspended(
		context.Background(),
		&api.SetServiceSuspendedReq{Suspended: true},
		api.SetServiceSuspendedParams{},
	)

	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestDeleteService(t *testing.T) {
	ctx, users, _ := usercontext.NewTestUserContext(&usercontext.User{Username: "alice"})

	res, err := NewInstallController(&lifecycleStub{}, users).
		DeleteService(ctx, api.DeleteServiceParams{ReleaseId: "release-id", XOnyxiaProject: "user-alice"})
	require.NoError(t, err)
	assert.IsType(t, &api.DeleteServiceNoContent{}, res)

	res, err = NewInstallController(&lifecycleStub{deleteErr: domain.ErrNotFound}, users).
		DeleteService(ctx, api.DeleteServiceParams{})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	anonymous, _ := usercontext.NewUserContext()
	res, err = NewInstallController(&lifecycleStub{}, anonymous).
		DeleteService(context.Background(), api.DeleteServiceParams{})
	assert.Nil(t, res)
	assert.ErrorIs(t, err, domain.ErrForbidden)
}
