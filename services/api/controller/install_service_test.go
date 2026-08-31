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
	start      func(context.Context, domain.StartRequest) (domain.StartResponse, error)
	suspendErr error
	resumeErr  error
	deleteErr  error
}

func (s *lifecycleStub) Start(
	ctx context.Context,
	req domain.StartRequest,
) (domain.StartResponse, error) {
	if s.start == nil {
		return domain.StartResponse{}, nil
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

func TestInstallServiceSelectsPackageVersionAndCanonicalReleaseID(t *testing.T) {
	tests := []struct {
		name           string
		packageVersion api.OptString
		legacyVersion  api.OptString
		wantVersion    string
	}{
		{
			name:           "packageVersion takes precedence",
			packageVersion: api.NewOptString("2.0.0"),
			legacyVersion:  api.NewOptString("1.0.0"),
			wantVersion:    "2.0.0",
		},
		{
			name:          "legacy version remains supported",
			legacyVersion: api.NewOptString("1.0.0"),
			wantVersion:   "1.0.0",
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
			) (domain.StartResponse, error) {
				captured = req
				return domain.StartResponse{}, nil
			}}
			ctrl := NewInstallController(lifecycle, users)

			res, err := ctrl.InstallService(ctx, &api.ServiceInstallRequest{
				CatalogId:      "catalog",
				PackageName:    "jupyter",
				PackageVersion: tt.packageVersion,
				Version:        tt.legacyVersion,
				Options:        api.ServiceInstallRequestOptions{},
				Name:           "display-name",
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

func TestMapSetServiceSuspendedError(t *testing.T) {
	assert.IsType(t, &api.SetServiceSuspendedNotFound{}, mapSetServiceSuspendedError(domain.ErrNotFound))
	assert.IsType(
		t,
		&api.SetServiceSuspendedUnprocessableEntity{},
		mapSetServiceSuspendedError(domain.ErrNotSupported),
	)
	assert.IsType(
		t,
		&api.SetServiceSuspendedInternalServerError{},
		mapSetServiceSuspendedError(errors.New("unexpected")),
	)
}
