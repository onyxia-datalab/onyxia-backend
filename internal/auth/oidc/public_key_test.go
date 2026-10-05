package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

// encodePublicKey returns the key in onyxia-api's oidc.public-key format:
// base64 of the DER-encoded X.509 SubjectPublicKeyInfo.
func encodePublicKey(t *testing.T, key any) string {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(der)
}

func signRS256(t *testing.T, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, nil)
	require.NoError(t, err)
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	require.NoError(t, err)
	return raw
}

func newPublicKeyAuth(t *testing.T, publicKey string) *Auth {
	t.Helper()
	_, writer := usercontext.NewUserContext()
	// The issuer is unreachable on purpose: in publicKey mode it must not be
	// contacted.
	a, err := New(context.Background(), OIDCConfig{
		IssuerURI:     "https://unreachable.invalid/realms/onyxia",
		PublicKey:     publicKey,
		UsernameClaim: "preferred_username",
	}, writer)
	require.NoError(t, err)
	return a
}

func TestPublicKey_AcceptsTokenSignedWithKey(t *testing.T) {
	key := newRSAKey(t)
	a := newPublicKeyAuth(t, encodePublicKey(t, &key.PublicKey))

	token := signRS256(t, key, map[string]any{
		// Like onyxia-api, the issuer is not checked in this mode.
		"iss":                "https://another-issuer.example.org",
		"preferred_username": "alice",
		"exp":                time.Now().Add(time.Hour).Unix(),
	})

	_, err := a.Verifier.Verify(context.Background(), token)
	require.NoError(t, err)
}

func TestPublicKey_RejectsTokenSignedWithOtherKey(t *testing.T) {
	a := newPublicKeyAuth(t, encodePublicKey(t, &newRSAKey(t).PublicKey))

	token := signRS256(t, newRSAKey(t), map[string]any{
		"preferred_username": "alice",
		"exp":                time.Now().Add(time.Hour).Unix(),
	})

	_, err := a.Verifier.Verify(context.Background(), token)
	assert.Error(t, err)
}

func TestPublicKey_RejectsExpiredToken(t *testing.T) {
	key := newRSAKey(t)
	a := newPublicKeyAuth(t, encodePublicKey(t, &key.PublicKey))

	token := signRS256(t, key, map[string]any{
		"preferred_username": "alice",
		"exp":                time.Now().Add(-time.Hour).Unix(),
	})

	_, err := a.Verifier.Verify(context.Background(), token)
	assert.ErrorContains(t, err, "expired")
}

func TestPublicKey_InvalidKeyFailsStartup(t *testing.T) {
	_, writer := usercontext.NewUserContext()

	tests := map[string]string{
		"not base64":      "%%%not-base64%%%",
		"not a DER key":   base64.StdEncoding.EncodeToString([]byte("garbage")),
		"non RSA (ECDSA)": encodePublicKey(t, newECKey(t).Public()),
	}

	for name, publicKey := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := New(context.Background(), OIDCConfig{PublicKey: publicKey}, writer)
			assert.Error(t, err)
		})
	}
}
