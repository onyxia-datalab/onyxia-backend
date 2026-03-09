package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/internal/auth"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	oas "github.com/onyxia-datalab/onyxia-backend/onboarding/api/oas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSecurityHandler_NoAuthMode_SetsAnonymousUserInContext(t *testing.T) {
	t.Parallel()

	reader, writer := usercontext.NewUserContext()

	sec, err := BuildSecurityHandler(
		context.Background(),
		"none",
		OIDCConfigOnboarding{},
		writer,
	)
	require.NoError(t, err)
	require.NotNil(t, sec)

	req := httptest.NewRequest("GET", "http://example.com/api/onboarding", nil)
	ctx, err := sec.HandleOidc(
		context.Background(),
		oas.OperationName("test-operation"),
		oas.Oidc{Request: req},
	)
	require.NoError(t, err)

	user, ok := reader.GetUser(ctx)
	require.True(t, ok, "expected user in context")
	assert.Equal(t, "anonymous", user.Username)
	assert.Nil(t, user.Groups)
	assert.Nil(t, user.Roles)
	assert.NotNil(t, user.Attributes)
}

func TestBuildSecurityHandler_OIDCModeWithEmptyConfig_ReturnsError(t *testing.T) {
	t.Parallel()

	_, writer := usercontext.NewUserContext()

	sec, err := BuildSecurityHandler(
		context.Background(),
		"oidc",
		OIDCConfigOnboarding{},
		writer,
	)

	assert.Error(t, err)
	assert.Nil(t, sec, "expected nil security handler on error")
}

type stubAuth struct {
	called    bool
	gotOp     oas.OperationName
	returnCtx context.Context
	returnErr error
}

func (s *stubAuth) VerifyRequest(
	ctx context.Context,
	op oas.OperationName,
	_ *http.Request,
) (context.Context, error) {
	s.called = true
	s.gotOp = op
	if s.returnCtx != nil {
		return s.returnCtx, s.returnErr
	}
	return ctx, s.returnErr
}

var _ auth.Auth = (*stubAuth)(nil)

func TestSecurityAdapter_HandleOidc_DelegatesToAuthHandler(t *testing.T) {
	t.Parallel()

	base := context.Background()
	wantCtx := context.WithValue(base, "k", "v")

	stub := &stubAuth{returnCtx: wantCtx}
	sec := newSecurityAdapter(stub)

	req := httptest.NewRequest("POST", "http://example.com/api/onboarding?x=1", nil)
	req.Header.Set("Authorization", "DPoP tkn")
	req.Header.Set("DPoP", "proof")

	gotCtx, err := sec.HandleOidc(
		base,
		oas.OperationName("op"),
		oas.Oidc{Request: req},
	)

	require.NoError(t, err)
	require.True(t, stub.called, "VerifyRequest should be called")
	assert.Equal(t, oas.OperationName("op"), stub.gotOp)
	assert.Equal(t, wantCtx, gotCtx)
}

func TestSecurityAdapter_HandleOidc_PropagatesError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	stub := &stubAuth{returnErr: wantErr}
	sec := newSecurityAdapter(stub)

	req := httptest.NewRequest("POST", "http://example.com/api/onboarding", nil)
	gotCtx, err := sec.HandleOidc(
		context.Background(),
		oas.OperationName("op"),
		oas.Oidc{Request: req},
	)

	assert.ErrorIs(t, err, wantErr)
	assert.NotNil(t, gotCtx)
}
