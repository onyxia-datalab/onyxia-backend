package oidc

import "context"

type cnfJKTKey struct{}

// WithCnfJKT stores the cnf.jkt thumbprint in context.
func WithCnfJKT(ctx context.Context, jkt string) context.Context {
	return context.WithValue(ctx, cnfJKTKey{}, jkt)
}

// CnfJKTFromContext extracts the cnf.jkt thumbprint from context.
func CnfJKTFromContext(ctx context.Context) (string, bool) {
	jkt, ok := ctx.Value(cnfJKTKey{}).(string)
	return jkt, ok && jkt != ""
}
