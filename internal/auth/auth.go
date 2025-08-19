package auth

import "context"

type Handler interface {
	Authenticate(ctx context.Context, operation, token string) (context.Context, error)
}
