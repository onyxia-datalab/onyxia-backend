package oidc

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/onyxia-datalab/onyxia-backend/internal/auth"
	"github.com/onyxia-datalab/onyxia-backend/internal/auth/oidc/dpop"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
)

type TokenVerifier interface {
	Verify(ctx context.Context, token string) (*oidc.IDToken, error)
}

type OIDCConfig struct {
	IssuerURI     string
	SkipTLSVerify bool
	ClientID      string
	Audience      string
	UsernameClaim string
	GroupsClaim   string
	RolesClaim    string
}

type Auth struct {
	UsernameClaim string
	GroupsClaim   string
	RolesClaim    string
	Verifier      TokenVerifier
	Audience      string
	Writer        usercontext.Writer
	JTICache      *dpop.JTICache
}

var _ auth.Auth = (*Auth)(nil)

func New(ctx context.Context, cfg OIDCConfig, writer usercontext.Writer) (*Auth, error) {
	provider, err := oidc.NewProvider(ctx, cfg.IssuerURI)
	if err != nil {
		slog.Error(
			"Failed to init OIDC provider",
			slog.String("issuer", cfg.IssuerURI),
			slog.Any("error", err),
		)
		return nil, err
	}
	verifier := provider.Verifier(&oidc.Config{
		ClientID:                   cfg.ClientID,
		InsecureSkipSignatureCheck: cfg.SkipTLSVerify,
	})
	if cfg.Audience == "" {
		slog.Warn("Skipping audience validation because 'audience' is empty")
	}
	slog.Info("OIDC Initialized",
		slog.String("issuer", cfg.IssuerURI),
		slog.String("client_id", cfg.ClientID),
		slog.String("aud", cfg.Audience),
	)
	return &Auth{
		UsernameClaim: cfg.UsernameClaim,
		GroupsClaim:   cfg.GroupsClaim,
		RolesClaim:    cfg.RolesClaim,
		Verifier:      verifier,
		Audience:      cfg.Audience,
		Writer:        writer,
		JTICache:      dpop.NewJTICache(),
	}, nil
}

func (a *Auth) VerifyRequest(
	ctx context.Context,
	operation string,
	r *http.Request,
) (context.Context, error) {
	tokenStr, isDPoP := dpop.FindAuthorization(r.Header, "DPoP")
	if !isDPoP {
		var ok bool
		tokenStr, ok = dpop.FindAuthorization(r.Header, "Bearer")
		if !ok {
			return ctx, fmt.Errorf("missing authorization header")
		}
	}

	info, err := a.verifyToken(ctx, operation, tokenStr)
	if err != nil {
		return ctx, err
	}

	if info.cnfJKT != "" {
		if !isDPoP {
			return ctx, fmt.Errorf("token requires DPoP binding")
		}
		proof := r.Header.Get("DPoP")
		if proof == "" {
			return ctx, fmt.Errorf("missing dpop proof")
		}
		if err := a.verifyProof(proof, r.Method, dpop.AbsoluteRequestURL(r), tokenStr, info.cnfJKT); err != nil {
			return ctx, err
		}
	}

	return a.applyUserContext(ctx, info), nil
}

type tokenInfo struct {
	username string
	groups   []string
	roles    []string
	attrs    map[string]any
	cnfJKT   string
}

func (a *Auth) verifyToken(
	ctx context.Context,
	operation string,
	tokenStr string,
) (*tokenInfo, error) {
	slog.Info("Verifying OIDC Token", slog.String("operation", operation))

	token, err := a.Verifier.Verify(ctx, tokenStr)
	if err != nil {
		slog.Error(
			"OIDC Token Verification Failed",
			slog.String("operation", operation),
			slog.Any("error", err),
		)
		return nil, err
	}

	var claims map[string]any
	if err := token.Claims(&claims); err != nil {
		slog.Error("Failed to extract claims from token", slog.Any("error", err))
		return nil, err
	}

	if err := a.validateAudience(claims); err != nil {
		return nil, err
	}

	username, err := a.extractClaim(claims, a.UsernameClaim)
	if err != nil {
		return nil, err
	}

	groups := a.extractStringArray(claims, a.GroupsClaim)
	roles := a.extractStringArray(claims, a.RolesClaim)

	slog.Info("OIDC Authentication Successful",
		slog.String("user", username),
		slog.String("operation", operation),
		slog.Any("groups", groups),
		slog.Any("roles", roles),
	)

	filtered := make(map[string]any, len(claims))
	for k, v := range claims {
		if k != a.UsernameClaim && k != a.GroupsClaim && k != a.RolesClaim {
			filtered[k] = v
		}
	}

	jkt, _ := extractCnfJKT(claims)
	return &tokenInfo{
		username: username,
		groups:   groups,
		roles:    roles,
		attrs:    filtered,
		cnfJKT:   jkt,
	}, nil
}

func (a *Auth) applyUserContext(ctx context.Context, info *tokenInfo) context.Context {
	if info.cnfJKT != "" {
		ctx = WithCnfJKT(ctx, info.cnfJKT)
	}
	return a.Writer.WithUser(ctx, &usercontext.User{
		Username:   info.username,
		Groups:     info.groups,
		Roles:      info.roles,
		Attributes: info.attrs,
	})
}

func (a *Auth) verifyProof(
	proof string,
	method string,
	url string,
	accessToken string,
	expectedJKT string,
) error {
	if expectedJKT == "" {
		return fmt.Errorf("missing cnf.jkt claim")
	}
	jkt, err := dpop.VerifyProof(proof, method, url, accessToken, time.Now(), a.JTICache.Seen)
	if err != nil {
		return err
	}
	if jkt != expectedJKT {
		return fmt.Errorf("dpop jkt mismatch")
	}
	return nil
}

func (a *Auth) validateAudience(claims map[string]any) error {
	if a.Audience == "" {
		return nil
	}
	aud, ok := claims["aud"]
	if !ok {
		slog.Error("Missing audience claim")
		return fmt.Errorf("missing audience claim")
	}
	switch v := aud.(type) {
	case string:
		if v != a.Audience {
			slog.Error("Invalid audience", slog.String("expected", a.Audience), slog.String("got", v))
			return fmt.Errorf("invalid audience: expected %q, got %q", a.Audience, v)
		}
	case []string:
		if !slices.Contains(v, a.Audience) {
			slog.Error("Invalid audience", slog.String("expected", a.Audience), slog.Any("got", v))
			return fmt.Errorf("invalid audience: expected %q, got %v", a.Audience, v)
		}
	case []any:
		ss := make([]string, len(v))
		for i, it := range v {
			s, ok := it.(string)
			if !ok {
				slog.Error("Audience element is not a string", slog.Any("item", it))
				return fmt.Errorf("audience element is not a string: %v", it)
			}
			ss[i] = s
		}
		if !slices.Contains(ss, a.Audience) {
			slog.Error("Invalid audience", slog.String("expected", a.Audience), slog.Any("got", ss))
			return fmt.Errorf("invalid audience: expected %q, got %v", a.Audience, ss)
		}
	default:
		slog.Error("Unexpected audience format", slog.Any("aud", v))
		return fmt.Errorf("invalid audience format")
	}
	return nil
}

func (a *Auth) extractClaim(claims map[string]any, name string) (string, error) {
	v, ok := claims[name]
	if !ok {
		slog.Error("Missing required claim", slog.String("claim", name))
		return "", fmt.Errorf("missing %q claim", name)
	}
	s, ok := v.(string)
	if !ok {
		slog.Error("Unexpected claim format", slog.String("claim", name))
		return "", fmt.Errorf("unknown format for claim %q", name)
	}
	return s, nil
}

func (a *Auth) extractStringArray(claims map[string]any, name string) []string {
	if name == "" {
		return nil
	}
	v, ok := claims[name]
	if !ok {
		slog.Warn("Claim not found", slog.String("claim", name))
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		slog.Warn("Unexpected format for claim", slog.String("claim", name), slog.Any("value", v))
		return nil
	}
	out := make([]string, len(arr))
	for i, it := range arr {
		s, ok := it.(string)
		if !ok {
			slog.Warn(
				"Non-string element in claim -> discard whole array",
				slog.String("claim", name),
				slog.Any("value", it),
			)
			return nil
		}
		out[i] = s
	}
	return out
}

func extractCnfJKT(claims map[string]any) (string, bool) {
	cnf, ok := claims["cnf"].(map[string]any)
	if !ok {
		return "", false
	}
	jkt, ok := cnf["jkt"].(string)
	if !ok || jkt == "" {
		return "", false
	}
	return jkt, true
}
