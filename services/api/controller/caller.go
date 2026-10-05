package controller

import (
	"context"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
)

// callerFrom returns the authenticated caller of a request. The security
// handler stores it in the context of every authenticated operation; its
// absence is a wiring error, reported as a denial.
func callerFrom(ctx context.Context, users usercontext.UserGetter) (usercontext.User, error) {
	u, ok := users.GetUser(ctx)
	if !ok || u == nil {
		slog.ErrorContext(ctx, "user not found in context")
		return usercontext.User{}, domain.ErrForbidden
	}
	return *u, nil
}
