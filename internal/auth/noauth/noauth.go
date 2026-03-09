package noauth

import (
	"context"
	"net/http"

	"github.com/onyxia-datalab/onyxia-backend/internal/auth"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
)

type Handler struct {
	Writer usercontext.Writer
}

var _ auth.Auth = (*Handler)(nil)

func (h *Handler) VerifyRequest(ctx context.Context, _ string, _ *http.Request) (context.Context, error) {
	return h.withAnonymous(ctx), nil
}

func (h *Handler) withAnonymous(ctx context.Context) context.Context {
	return h.Writer.WithUser(ctx, &usercontext.User{
		Username:   "anonymous",
		Groups:     nil,
		Roles:      nil,
		Attributes: map[string]any{},
	})
}
