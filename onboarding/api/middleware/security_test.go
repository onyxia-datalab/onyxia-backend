package middleware

import (
	"context"
	"errors"
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

	ctx, err := sec.HandleBearer(
		context.Background(),
		oas.OperationName("test-operation"),
		oas.Bearer{Token: "ignored"},
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
		OIDCConfigOnboarding{}, // issuer vide etc.
		writer,
	)

	assert.Error(t, err)
	assert.Nil(t, sec, "expected nil security handler on error")
}

type stubAuthHandler struct {
	called    bool
	gotOp     oas.OperationName
	gotToken  string
	returnCtx context.Context
	returnErr error
}

func (s *stubAuthHandler) Authenticate(
	ctx context.Context,
	op oas.OperationName,
	token string,
) (context.Context, error) {
	s.called = true
	s.gotOp = op
	s.gotToken = token
	if s.returnCtx != nil {
		return s.returnCtx, s.returnErr
	}
	return ctx, s.returnErr
}

var _ auth.Handler = (*stubAuthHandler)(nil)

func TestSecurityAdapter_HandleBearer_DelegatesToAuthHandler(t *testing.T) {
	t.Parallel()

	base := context.Background()
	wantCtx := context.WithValue(base, "k", "v")

	stub := &stubAuthHandler{
		returnCtx: wantCtx,
	}
	sec := newSecurityAdapter(stub)

	gotCtx, err := sec.HandleBearer(
		base,
		oas.OperationName("op"),
		oas.Bearer{Token: "tkn"},
	)
	require.NoError(t, err)

	require.True(t, stub.called, "Authenticate should be called")
	assert.Equal(t, oas.OperationName("op"), stub.gotOp)
	assert.Equal(t, "tkn", stub.gotToken)
	assert.Equal(t, wantCtx, gotCtx)
}

func TestSecurityAdapter_HandleBearer_PropagatesError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	stub := &stubAuthHandler{returnErr: wantErr}
	sec := newSecurityAdapter(stub)

	gotCtx, err := sec.HandleBearer(
		context.Background(),
		oas.OperationName("op"),
		oas.Bearer{Token: "tkn"},
	)

	assert.ErrorIs(t, err, wantErr)
	// ctx peut être celui d'entrée (stub le renvoie tel quel si returnCtx nil)
	assert.NotNil(t, gotCtx)
}

func TestSecurityAdapter_HandleDpop_ReturnsCtxAndNil_ForNow(t *testing.T) {
	t.Parallel()

	sec := newSecurityAdapter(&stubAuthHandler{})

	req := httptest.NewRequest("POST", "http://example.com/api/onboarding", nil)
	ctx := context.WithValue(context.Background(), "ctx", "val")

	gotCtx, err := sec.HandleDpop(
		ctx,
		oas.OperationName("op"),
		oas.Dpop{
			Request: req,
		},
	)

	require.NoError(t, err)
	assert.Equal(t, ctx, gotCtx)
}
