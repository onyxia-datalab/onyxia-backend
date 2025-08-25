package middleware

import (
	"context"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	oas "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
	"github.com/stretchr/testify/assert"
)

func TestBuildSecurityHandler_NoAuthMode(t *testing.T) {
	reader, writer := usercontext.NewUserContext()

	sec, err := BuildSecurityHandler(
		context.Background(),
		"none",
		OIDCConfigOnboarding{},
		writer,
	)
	assert.NoError(t, err)
	assert.NotNil(t, sec)

	ctx, err := sec.HandleOidc(context.Background(), "test-operation", oas.Oidc{Token: "ignored"})
	assert.NoError(t, err)

	user, ok := reader.GetUser(ctx)
	assert.True(t, ok, "expected user in context")
	assert.Equal(t, "anonymous", user.Username)
	assert.Nil(t, user.Groups)
	assert.Nil(t, user.Roles)
	assert.NotNil(t, user.Attributes)
}
