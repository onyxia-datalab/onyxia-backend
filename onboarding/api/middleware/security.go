package middleware

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/ogen-go/ogen/ogenerrors"
	"github.com/onyxia-datalab/onyxia-backend/internal/auth"
	"github.com/onyxia-datalab/onyxia-backend/internal/auth/noauth"
	"github.com/onyxia-datalab/onyxia-backend/internal/auth/oidc"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	oas "github.com/onyxia-datalab/onyxia-backend/onboarding/api/oas"
)

type securityAdapter struct{ h auth.Handler }

func newSecurityAdapter(h auth.Handler) *securityAdapter { return &securityAdapter{h: h} }

var _ oas.SecurityHandler = (*securityAdapter)(nil)

func (a *securityAdapter) HandleBearerSchema(
	ctx context.Context,
	operation oas.OperationName,
	req oas.BearerSchema,
) (context.Context, error) {
	return a.h.Authenticate(ctx, operation, req.Token)
}

func (a *securityAdapter) HandleDpopSchema(
	ctx context.Context,
	operation oas.OperationName,
	req oas.DpopSchema,
) (context.Context, error) {
	scheme, token, ok := strings.Cut(req.APIKey, " ")
	if !ok || !strings.EqualFold(scheme, "DPoP") {
		//If Authorization scheme is not DPoP, skip this security handler
		return ctx, ogenerrors.ErrSkipServerSecurity
	}
	if token == "" {
		return ctx, errors.New("invalid DPoP authorization scheme")
	}
	return a.h.Authenticate(ctx, operation, token)
}

func (a *securityAdapter) HandleDpopProof(
	ctx context.Context,
	operation oas.OperationName,
	_ oas.DpopProof,
) (context.Context, error) {
	// DPoP proof is optional; validation may be added later.
	return ctx, nil
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
