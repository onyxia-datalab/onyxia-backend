package usercontext

import "context"

type ctxKey struct{}

var ctxUserKey ctxKey

type readerImpl struct{}
type writerImpl struct{}

func NewUserContext() (Reader, Writer) {
	return readerImpl{}, writerImpl{}
}

// ==== Writer ====

func (writerImpl) WithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, ctxUserKey, u)

} // ==== Reader ====

func (readerImpl) GetUser(ctx context.Context) (*User, bool) {
	val := ctx.Value(ctxUserKey)
	if val == nil {
		return nil, false
	}
	u, ok := val.(*User)
	return u, ok && u != nil
}

func (r readerImpl) GetUsername(ctx context.Context) (string, bool) {
	if u, ok := r.GetUser(ctx); ok && u != nil && u.Username != "" {
		return u.Username, true
	}
	return "", false
}

func (r readerImpl) GetGroups(ctx context.Context) ([]string, bool) {
	if u, ok := r.GetUser(ctx); ok && u != nil && len(u.Groups) > 0 {
		return u.Groups, true
	}
	return nil, false
}

func (r readerImpl) GetRoles(ctx context.Context) ([]string, bool) {
	if u, ok := r.GetUser(ctx); ok && u != nil && len(u.Roles) > 0 {
		return u.Roles, true
	}
	return nil, false
}

func (r readerImpl) GetAttributes(ctx context.Context) (map[string]any, bool) {
	if u, ok := r.GetUser(ctx); ok && u != nil && u.Attributes != nil {
		return u.Attributes, true
	}
	return nil, false
}
