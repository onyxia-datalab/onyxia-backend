package usercontext

import "context"

type Reader interface {
	GetUser(ctx context.Context) (*User, bool)
	GetUsername(ctx context.Context) (string, bool)
	GetGroups(ctx context.Context) ([]string, bool)
	GetRoles(ctx context.Context) ([]string, bool)
	GetAttributes(ctx context.Context) (map[string]any, bool)
}

type Writer interface {
	WithUser(ctx context.Context, u *User) context.Context
}
