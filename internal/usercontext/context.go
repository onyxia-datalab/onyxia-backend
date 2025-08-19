package usercontext

import "context"

type ctxKey struct{ name string }

var ctxUserKey = &ctxKey{"user"}

type userContext struct{}

func (userContext) GetUser(ctx context.Context) (*User, bool) {
	u, ok := ctx.Value(ctxUserKey).(*User)
	return u, ok
}

func (uc userContext) GetUsername(ctx context.Context) (string, bool) {
	if u, ok := uc.GetUser(ctx); ok {
		return u.Username, true
	}
	return "", false
}

func (uc userContext) GetGroups(ctx context.Context) ([]string, bool) {
	if u, ok := uc.GetUser(ctx); ok {
		return u.Groups, true
	}
	return nil, false
}

func (uc userContext) GetRoles(ctx context.Context) ([]string, bool) {
	if u, ok := uc.GetUser(ctx); ok {
		return u.Roles, true
	}
	return nil, false
}

func (uc userContext) GetAttributes(ctx context.Context) (map[string]any, bool) {
	if u, ok := uc.GetUser(ctx); ok {
		return u.Attributes, true
	}
	return nil, false
}

func (userContext) WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, ctxUserKey, u)
}
