package middleware

import (
	"context"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	oas "github.com/onyxia-datalab/onyxia-backend/onboarding/api/oas"
	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/stretchr/testify/assert"
)

func TestBuildSecurityHandlerNoAuthMode(t *testing.T) {
	reader, writer := usercontext.NewUserContext()

	sec, err := BuildSecurityHandler(
		context.Background(),
		"none",
		OIDCConfigOnboarding{},
		writer,
	)
	assert.NoError(t, err)
	assert.NotNil(t, sec)

	ctx, err := sec.HandleBearerSchema(
		context.Background(),
		"test-operation",
		oas.BearerSchema{Token: "ignored"},
	)
	assert.NoError(t, err)

	user, ok := reader.GetUser(ctx)
	assert.True(t, ok, "expected user in context")
	assert.Equal(t, "anonymous", user.Username)
	assert.Nil(t, user.Groups)
	assert.Nil(t, user.Roles)
	assert.NotNil(t, user.Attributes)
}

func TestHandleDpopSchemaSkipsNonDpopAuthorization(t *testing.T) {
	_, writer := usercontext.NewUserContext()

	sec, err := BuildSecurityHandler(
		context.Background(),
		"none",
		OIDCConfigOnboarding{},
		writer,
	)
	assert.NoError(t, err)
	assert.NotNil(t, sec)

	ctx, err := sec.HandleDpopSchema(
		context.Background(),
		"test-operation",
		oas.DpopSchema{APIKey: "Bearer ignored"},
	)
	assert.ErrorIs(t, err, ogenerrors.ErrSkipServerSecurity)
	assert.Equal(t, context.Background(), ctx)
}
