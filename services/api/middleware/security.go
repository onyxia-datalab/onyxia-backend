package middleware

import (
	"context"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/internal/auth"
	"github.com/onyxia-datalab/onyxia-backend/internal/auth/noauth"
	"github.com/onyxia-datalab/onyxia-backend/internal/auth/oidc"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	oas "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
)

type securityAdapter struct{ h auth.Handler }

func newSecurityAdapter(h auth.Handler) *securityAdapter { return &securityAdapter{h: h} }

var _ oas.SecurityHandler = (*securityAdapter)(nil)

func (a *securityAdapter) HandleOidc(
	ctx context.Context,
	operation string,
	req oas.Oidc,
) (context.Context, error) {
	return a.h.Authenticate(ctx, operation, req.Token)
}

type OIDCConfigOnboarding struct {
	IssuerURI     string
	SkipTLSVerify bool
	ClientID      string
	Audience      string
	UsernameClaim string
	GroupsClaim   string
	RolesClaim    string
}

func BuildSecurityHandler(
	ctx context.Context,
	authenticationMode string,
	cfg OIDCConfigOnboarding,
	writer usercontext.Writer,
) (oas.SecurityHandler, error) {

	if authenticationMode == "none" {
		slog.Warn("🚀 Running in No-Auth Mode")
		return newSecurityAdapter(&noauth.Handler{Writer: writer}), nil
	}

	shared := oidc.OIDCConfig{
		IssuerURI:     cfg.IssuerURI,
		SkipTLSVerify: cfg.SkipTLSVerify,
		ClientID:      cfg.ClientID,
		Audience:      cfg.Audience,
		UsernameClaim: cfg.UsernameClaim,
		GroupsClaim:   cfg.GroupsClaim,
		RolesClaim:    cfg.RolesClaim,
	}

	h, err := oidc.New(ctx, shared, writer)
	if err != nil {
		return nil, err
	}
	return newSecurityAdapter(h), nil
}
