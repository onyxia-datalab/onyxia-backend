package auth

import (
	"context"
	"net/http"
)

type Auth interface {
	VerifyRequest(ctx context.Context, operation string, r *http.Request) (context.Context, error)
}
