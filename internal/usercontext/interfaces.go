package usercontext

import "context"

type UserGetter interface {
	GetUser(ctx context.Context) (*User, bool)
}
type UsernameGetter interface {
	GetUsername(ctx context.Context) (string, bool)
}
type GroupsGetter interface {
	GetGroups(ctx context.Context) ([]string, bool)
}
type RolesGetter interface {
	GetRoles(ctx context.Context) ([]string, bool)
}
type AttributesGetter interface {
	GetAttributes(ctx context.Context) (map[string]any, bool)
}
type Reader interface {
	UserGetter
	UsernameGetter
	GroupsGetter
	RolesGetter
	AttributesGetter
}
type Writer interface {
	WithUser(ctx context.Context, u *User) context.Context
}
