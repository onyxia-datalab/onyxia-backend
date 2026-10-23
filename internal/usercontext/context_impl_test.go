package usercontext

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReaderReturnsStoredUser(t *testing.T) {
	user := DefaultTestUser()
	ctx, reader, _ := NewTestUserContext(user)

	got, ok := reader.GetUser(ctx)
	require.True(t, ok)
	assert.Same(t, user, got)

	username, ok := reader.GetUsername(ctx)
	assert.True(t, ok)
	assert.Equal(t, "test-user", username)

	groups, ok := reader.GetGroups(ctx)
	assert.True(t, ok)
	assert.Equal(t, []string{"test-group"}, groups)

	roles, ok := reader.GetRoles(ctx)
	assert.True(t, ok)
	assert.Equal(t, []string{"role1"}, roles)

	attrs, ok := reader.GetAttributes(ctx)
	assert.True(t, ok)
	assert.Equal(t, map[string]any{"attr1": "value1"}, attrs)
}

func TestReaderWithoutUser(t *testing.T) {
	reader, writer := NewUserContext()

	for name, ctx := range map[string]context.Context{
		"no user":  context.Background(),
		"nil user": writer.WithUser(context.Background(), nil),
	} {
		t.Run(name, func(t *testing.T) {
			u, ok := reader.GetUser(ctx)
			assert.False(t, ok)
			assert.Nil(t, u)

			_, ok = reader.GetUsername(ctx)
			assert.False(t, ok)
			_, ok = reader.GetGroups(ctx)
			assert.False(t, ok)
			_, ok = reader.GetRoles(ctx)
			assert.False(t, ok)
			_, ok = reader.GetAttributes(ctx)
			assert.False(t, ok)
		})
	}
}

// A user with empty fields is present, but each empty field is reported absent.
func TestReaderWithEmptyUserFields(t *testing.T) {
	ctx, reader, _ := NewTestUserContext(&User{})

	_, ok := reader.GetUser(ctx)
	assert.True(t, ok)
	_, ok = reader.GetUsername(ctx)
	assert.False(t, ok)
	_, ok = reader.GetGroups(ctx)
	assert.False(t, ok)
	_, ok = reader.GetRoles(ctx)
	assert.False(t, ok)
	_, ok = reader.GetAttributes(ctx)
	assert.False(t, ok)
}
