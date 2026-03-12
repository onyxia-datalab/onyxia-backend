package oidc

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http/httptest"
	"testing"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/onyxia-datalab/onyxia-backend/internal/auth/oidc/dpop"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- helpers ---

func newECKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

func jktOf(t *testing.T, key *ecdsa.PrivateKey) string {
	t.Helper()
	jwk := jose.JSONWebKey{Key: key.Public(), Algorithm: string(jose.ES256)}
	thumb, err := jwk.Thumbprint(crypto.SHA256)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(thumb)
}

// buildToken creates a signed JWT whose payload contains the given claims.
// With InsecureSkipSignatureCheck the verifier only base64-decodes the payload,
// so any signing key is acceptable.
func buildToken(t *testing.T, key *ecdsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: key},
		new(jose.SignerOptions).WithHeader(jose.HeaderType, "JWT"),
	)
	require.NoError(t, err)
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	require.NoError(t, err)
	return raw
}

// buildProof creates a DPoP proof for the given method/URL/access-token.
func buildProof(t *testing.T, key *ecdsa.PrivateKey, htm, htu, accessToken string) string {
	t.Helper()
	pubJWK := jose.JSONWebKey{Key: key.Public(), Algorithm: string(jose.ES256)}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: key},
		(&jose.SignerOptions{}).
			WithHeader(jose.HeaderType, "dpop+jwt").
			WithHeader("jwk", pubJWK),
	)
	require.NoError(t, err)
	h := sha256.Sum256([]byte(accessToken))
	ath := base64.RawURLEncoding.EncodeToString(h[:])
	raw, err := jwt.Signed(signer).Claims(map[string]any{
		"htm": htm,
		"htu": htu,
		"iat": time.Now().Unix(),
		"jti": base64.RawURLEncoding.EncodeToString([]byte(htm + htu + accessToken)),
		"ath": ath,
	}).Serialize()
	require.NoError(t, err)
	return raw
}

type testAuth struct {
	*Auth
	reader usercontext.Reader
}

func newTestAuth(t *testing.T) *testAuth {
	t.Helper()
	reader, writer := usercontext.NewUserContext()
	verifier := gooidc.NewVerifier("https://fake-issuer", nil, &gooidc.Config{
		SkipClientIDCheck:          true,
		SkipExpiryCheck:            true,
		SkipIssuerCheck:            true,
		InsecureSkipSignatureCheck: true,
		SupportedSigningAlgs:       []string{"ES256"},
	})
	return &testAuth{
		Auth: &Auth{
			UsernameClaim: "preferred_username",
			GroupsClaim:   "groups",
			RolesClaim:    "roles",
			Verifier:      verifier,
			Writer:        writer,
			JTICache:      dpop.NewJTICache(),
		},
		reader: reader,
	}
}

// --- VerifyRequest tests ---

func TestVerifyRequest_MissingAuthHeader(t *testing.T) {
	t.Parallel()
	a := newTestAuth(t)
	req := httptest.NewRequest("GET", "http://example.com/api", nil)

	_, err := a.VerifyRequest(context.Background(), "op", req)

	assert.ErrorContains(t, err, "missing authorization header")
}

func TestVerifyRequest_Bearer_Valid(t *testing.T) {
	t.Parallel()
	key := newECKey(t)
	a := newTestAuth(t)
	token := buildToken(t, key, map[string]any{
		"preferred_username": "alice",
		"groups":             []any{"g1"},
		"roles":              []any{"admin"},
	})
	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	ctx, err := a.VerifyRequest(context.Background(), "op", req)

	require.NoError(t, err)
	user, ok := a.reader.GetUser(ctx)
	assert.True(t, ok)
	assert.Equal(t, "alice", user.Username)
}

// Un token DPoP-bound (cnf.jkt présent) ne doit PAS être accepté via Bearer.
func TestVerifyRequest_Bearer_DPoPBoundToken_Rejected(t *testing.T) {
	t.Parallel()
	key := newECKey(t)
	dpopKey := newECKey(t)
	a := newTestAuth(t)
	token := buildToken(t, key, map[string]any{
		"preferred_username": "alice",
		"cnf":                map[string]any{"jkt": jktOf(t, dpopKey)},
	})
	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	_, err := a.VerifyRequest(context.Background(), "op", req)

	assert.ErrorContains(t, err, "token requires DPoP binding")
}

func TestVerifyRequest_DPoP_Valid(t *testing.T) {
	t.Parallel()
	key := newECKey(t)
	dpopKey := newECKey(t)
	a := newTestAuth(t)
	jkt := jktOf(t, dpopKey)
	const htu = "http://example.com/api/resource"
	token := buildToken(t, key, map[string]any{
		"preferred_username": "alice",
		"cnf":                map[string]any{"jkt": jkt},
	})
	proof := buildProof(t, dpopKey, "POST", htu, token)
	req := httptest.NewRequest("POST", htu, nil)
	req.Header.Set("Authorization", "DPoP "+token)
	req.Header.Set("DPoP", proof)

	ctx, err := a.VerifyRequest(context.Background(), "op", req)

	require.NoError(t, err)
	user, ok := a.reader.GetUser(ctx)
	assert.True(t, ok)
	assert.Equal(t, "alice", user.Username)
}

func TestVerifyRequest_DPoP_MissingProofHeader(t *testing.T) {
	t.Parallel()
	key := newECKey(t)
	dpopKey := newECKey(t)
	a := newTestAuth(t)
	token := buildToken(t, key, map[string]any{
		"preferred_username": "alice",
		"cnf":                map[string]any{"jkt": jktOf(t, dpopKey)},
	})
	req := httptest.NewRequest("POST", "http://example.com/api", nil)
	req.Header.Set("Authorization", "DPoP "+token)
	// No DPoP proof header

	_, err := a.VerifyRequest(context.Background(), "op", req)

	assert.ErrorContains(t, err, "missing dpop proof")
}

func TestVerifyRequest_DPoP_WrongKey_JKTMismatch(t *testing.T) {
	t.Parallel()
	key := newECKey(t)
	dpopKey := newECKey(t)
	wrongKey := newECKey(t)
	a := newTestAuth(t)
	const htu = "http://example.com/api/resource"
	// cnf.jkt points to dpopKey, but proof is signed by wrongKey
	token := buildToken(t, key, map[string]any{
		"preferred_username": "alice",
		"cnf":                map[string]any{"jkt": jktOf(t, dpopKey)},
	})
	proof := buildProof(t, wrongKey, "POST", htu, token)
	req := httptest.NewRequest("POST", htu, nil)
	req.Header.Set("Authorization", "DPoP "+token)
	req.Header.Set("DPoP", proof)

	_, err := a.VerifyRequest(context.Background(), "op", req)

	assert.ErrorContains(t, err, "jkt mismatch")
}

func TestVerifyRequest_DPoP_HTUMismatch(t *testing.T) {
	t.Parallel()
	key := newECKey(t)
	dpopKey := newECKey(t)
	a := newTestAuth(t)
	jkt := jktOf(t, dpopKey)
	token := buildToken(t, key, map[string]any{
		"preferred_username": "alice",
		"cnf":                map[string]any{"jkt": jkt},
	})
	// proof built for a different URL
	proof := buildProof(t, dpopKey, "POST", "http://example.com/other", token)
	req := httptest.NewRequest("POST", "http://example.com/api/resource", nil)
	req.Header.Set("Authorization", "DPoP "+token)
	req.Header.Set("DPoP", proof)

	_, err := a.VerifyRequest(context.Background(), "op", req)

	assert.ErrorContains(t, err, "htu mismatch")
}
