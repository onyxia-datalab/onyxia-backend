package logging

import (
	"context"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"go.uber.org/zap/exp/zapslog"
)

// SetupDefault installs the production logger as slog's default. Every
// record logged with a *Context variant is enriched with the caller's
// username, groups and roles read from users.
//
// The returned flush function must be called before the process exits.
func SetupDefault(users usercontext.Reader) (flush func() error, err error) {
	logger, flush, err := NewLogger(nil, UserAttrs(users), zapslog.WithCaller(true))
	if err != nil {
		return nil, err
	}
	slog.SetDefault(logger)
	return flush, nil
}

// UserAttrs returns an AttrFunc adding the username, groups and roles stored
// in the context by the authentication layer.
func UserAttrs(users usercontext.Reader) AttrFunc {
	return func(ctx context.Context) []slog.Attr {
		attrs := make([]slog.Attr, 0, 3)
		if u, ok := users.GetUsername(ctx); ok && u != "" {
			attrs = append(attrs, slog.String("username", u))
		}
		if g, ok := users.GetGroups(ctx); ok && len(g) > 0 {
			attrs = append(attrs, slog.Any("groups", g))
		}
		if r, ok := users.GetRoles(ctx); ok && len(r) > 0 {
			attrs = append(attrs, slog.Any("roles", r))
		}
		return attrs
	}
}
