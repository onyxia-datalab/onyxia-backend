package dpop

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key
}

func ath(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

type proofOpts struct {
	key    *ecdsa.PrivateKey
	typ    string
	claims proofClaims
}

func buildProof(t *testing.T, o proofOpts) string {
	t.Helper()
	pubJWK := jose.JSONWebKey{Key: o.key.Public(), Algorithm: string(jose.ES256)}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: o.key},
		(&jose.SignerOptions{}).
			WithHeader(jose.HeaderType, o.typ).
			WithHeader("jwk", pubJWK),
	)
	require.NoError(t, err)
	raw, err := jwt.Signed(signer).Claims(o.claims).Serialize()
	require.NoError(t, err)
	return raw
}

func validClaims(now time.Time, token string) proofClaims {
	return proofClaims{
		Htm: "POST",
		Htu: "https://example.com/api/resource",
		Iat: now.Unix(),
		Jti: "unique-jti-" + base64.RawURLEncoding.EncodeToString([]byte(now.String())),
		Ath: ath(token),
	}
}

func TestVerifyProof_Valid(t *testing.T) {
	key := newKey(t)
	now := time.Now()
	const token = "my-access-token"

	proof := buildProof(t, proofOpts{
		key:    key,
		typ:    "dpop+jwt",
		claims: validClaims(now, token),
	})

	jkt, err := VerifyProof(proof, "POST", "https://example.com/api/resource", token, now, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, jkt)
}

func TestVerifyProof_InvalidTyp(t *testing.T) {
	key := newKey(t)
	now := time.Now()
	const token = "my-access-token"

	proof := buildProof(t, proofOpts{
		key:    key,
		typ:    "JWT",
		claims: validClaims(now, token),
	})

	_, err := VerifyProof(proof, "POST", "https://example.com/api/resource", token, now, nil)
	assert.ErrorContains(t, err, "typ")
}

func TestVerifyProof_HtmMismatch(t *testing.T) {
	key := newKey(t)
	now := time.Now()
	const token = "my-access-token"

	claims := validClaims(now, token)
	claims.Htm = "GET"
	proof := buildProof(t, proofOpts{key: key, typ: "dpop+jwt", claims: claims})

	_, err := VerifyProof(proof, "POST", "https://example.com/api/resource", token, now, nil)
	assert.ErrorContains(t, err, "htm")
}

func TestVerifyProof_HtuMismatch(t *testing.T) {
	key := newKey(t)
	now := time.Now()
	const token = "my-access-token"

	claims := validClaims(now, token)
	claims.Htu = "https://example.com/other"
	proof := buildProof(t, proofOpts{key: key, typ: "dpop+jwt", claims: claims})

	_, err := VerifyProof(proof, "POST", "https://example.com/api/resource", token, now, nil)
	assert.ErrorContains(t, err, "htu")
}

func TestVerifyProof_HtuCaseInsensitiveSchemeHost(t *testing.T) {
	key := newKey(t)
	now := time.Now()
	const token = "my-access-token"

	claims := validClaims(now, token)
	claims.Htu = "HTTPS://EXAMPLE.COM/api/resource"
	proof := buildProof(t, proofOpts{key: key, typ: "dpop+jwt", claims: claims})

	_, err := VerifyProof(proof, "POST", "https://example.com/api/resource", token, now, nil)
	assert.NoError(t, err)
}

func TestVerifyProof_Expired(t *testing.T) {
	key := newKey(t)
	now := time.Now()
	const token = "my-access-token"

	claims := validClaims(now, token)
	claims.Iat = now.Add(-(maxProofAge + 2*time.Second)).Unix()
	proof := buildProof(t, proofOpts{key: key, typ: "dpop+jwt", claims: claims})

	_, err := VerifyProof(proof, "POST", "https://example.com/api/resource", token, now, nil)
	assert.ErrorContains(t, err, "too old")
}

func TestVerifyProof_FutureIat(t *testing.T) {
	key := newKey(t)
	now := time.Now()
	const token = "my-access-token"

	claims := validClaims(now, token)
	claims.Iat = now.Add(maxClockSkew + 2*time.Second).Unix()
	proof := buildProof(t, proofOpts{key: key, typ: "dpop+jwt", claims: claims})

	_, err := VerifyProof(proof, "POST", "https://example.com/api/resource", token, now, nil)
	assert.ErrorContains(t, err, "future")
}

func TestVerifyProof_MissingJti(t *testing.T) {
	key := newKey(t)
	now := time.Now()
	const token = "my-access-token"

	claims := validClaims(now, token)
	claims.Jti = ""
	proof := buildProof(t, proofOpts{key: key, typ: "dpop+jwt", claims: claims})

	_, err := VerifyProof(proof, "POST", "https://example.com/api/resource", token, now, nil)
	assert.ErrorContains(t, err, "jti")
}

func TestVerifyProof_ReplayedJti(t *testing.T) {
	key := newKey(t)
	now := time.Now()
	const token = "my-access-token"

	proof := buildProof(t, proofOpts{
		key:    key,
		typ:    "dpop+jwt",
		claims: validClaims(now, token),
	})

	cache := NewJTICache()
	_, err := VerifyProof(proof, "POST", "https://example.com/api/resource", token, now, cache.Seen)
	require.NoError(t, err)

	_, err = VerifyProof(proof, "POST", "https://example.com/api/resource", token, now, cache.Seen)
	assert.ErrorContains(t, err, "replayed")
}

func TestVerifyProof_MissingAth(t *testing.T) {
	key := newKey(t)
	now := time.Now()
	const token = "my-access-token"

	claims := validClaims(now, token)
	claims.Ath = ""
	proof := buildProof(t, proofOpts{key: key, typ: "dpop+jwt", claims: claims})

	_, err := VerifyProof(proof, "POST", "https://example.com/api/resource", token, now, nil)
	assert.ErrorContains(t, err, "ath missing")
}

func TestVerifyProof_AthMismatch(t *testing.T) {
	key := newKey(t)
	now := time.Now()

	claims := validClaims(now, "token-A")
	proof := buildProof(t, proofOpts{key: key, typ: "dpop+jwt", claims: claims})

	_, err := VerifyProof(proof, "POST", "https://example.com/api/resource", "token-B", now, nil)
	assert.ErrorContains(t, err, "ath mismatch")
}

func TestVerifyProof_PrivateJWK(t *testing.T) {
	key := newKey(t)
	now := time.Now()
	const token = "my-access-token"

	// Build a proof where jwk contains the private key (should be rejected)
	privJWK := jose.JSONWebKey{Key: key, Algorithm: string(jose.ES256)}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: key},
		(&jose.SignerOptions{}).
			WithHeader(jose.HeaderType, "dpop+jwt").
			WithHeader("jwk", privJWK),
	)
	require.NoError(t, err)
	raw, err := jwt.Signed(signer).Claims(validClaims(now, token)).Serialize()
	require.NoError(t, err)

	_, err = VerifyProof(raw, "POST", "https://example.com/api/resource", token, now, nil)
	assert.ErrorContains(t, err, "public")
}

func TestJTICache_Eviction(t *testing.T) {
	cache := NewJTICache()

	// Manually insert an already-expired entry
	cache.mu.Lock()
	cache.entries["old-jti"] = time.Now().Add(-time.Second)
	cache.mu.Unlock()

	// Seen should evict it and return false (not a replay)
	assert.False(t, cache.Seen("old-jti"))
}
