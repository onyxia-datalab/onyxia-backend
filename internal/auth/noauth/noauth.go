package noauth

import (
	"context"

	"github.com/onyxia-datalab/onyxia-backend/internal/auth"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
)

type Handler struct {
	Writer usercontext.Writer
}

var _ auth.Handler = (*Handler)(nil)

func (h *Handler) Authenticate(ctx context.Context, operation, _ string) (context.Context, error) {
	return h.Writer.WithUser(ctx, &usercontext.User{
		Username:   "anonymous",
		Groups:     nil,
		Roles:      nil,
		Attributes: map[string]any{},
	}), nil
}
