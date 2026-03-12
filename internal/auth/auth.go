package auth

import (
	"context"
	"net/http"
)

type RequestVerifier interface {
	VerifyRequest(ctx context.Context, operation string, r *http.Request) (context.Context, error)
}
