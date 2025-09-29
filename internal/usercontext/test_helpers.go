package usercontext

import (
	"context"
)

func NewTestUserContext(
	u *User,
) (context.Context, Reader, Writer) {
	reader, writer := NewUserContext()
	ctx := writer.WithUser(context.Background(), u)

	return ctx, reader, writer
}

func DefaultTestUser() *User {
	return &User{
		Username: "test-user",
		Groups:   []string{"test-group"},
		Roles:    []string{"role1"},
		Attributes: map[string]any{
			"attr1": "value1",
		},
	}
}
