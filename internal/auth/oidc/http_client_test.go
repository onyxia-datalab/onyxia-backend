package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSelfSignedIssuer starts an IdP serving its discovery document over TLS
// with a certificate no client trusts by default.
func newSelfSignedIssuer(t *testing.T) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jwks" {
			_, _ = w.Write([]byte(`{"keys":[]}`))
			return
		}
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"jwks_uri":               srv.URL + "/jwks",
			"authorization_endpoint": srv.URL + "/auth",
			"token_endpoint":         srv.URL + "/token",
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNew_UntrustedIssuerCertificateIsRejected(t *testing.T) {
	srv := newSelfSignedIssuer(t)
	_, writer := usercontext.NewUserContext()

	_, err := New(context.Background(), OIDCConfig{IssuerURI: srv.URL}, writer)

	require.Error(t, err)
}

func TestNew_SkipTLSVerifyAcceptsUntrustedIssuerCertificate(t *testing.T) {
	srv := newSelfSignedIssuer(t)
	_, writer := usercontext.NewUserContext()

	a, err := New(context.Background(), OIDCConfig{IssuerURI: srv.URL, SkipTLSVerify: true}, writer)

	require.NoError(t, err)
	// Skipping TLS verification must never skip token signature checks.
	// The token is signed by a key the IdP does not publish.
	token := buildToken(t, newECKey(t), map[string]any{
		"iss": srv.URL, "sub": "alice", "exp": time.Now().Add(time.Hour).Unix(),
	})
	_, err = a.Verifier.Verify(context.Background(), token)
	assert.ErrorContains(t, err, "signature")
}
