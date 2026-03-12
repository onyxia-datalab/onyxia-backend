package dpop

import (
	"crypto"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const (
	maxProofAge  = 5 * time.Minute
	maxClockSkew = 60 * time.Second
	jtiTTL       = maxProofAge + maxClockSkew
)

type proofClaims struct {
	Htm string `json:"htm"`
	Htu string `json:"htu"`
	Iat int64  `json:"iat"`
	Jti string `json:"jti"`
	Ath string `json:"ath"`
}

// VerifyProof validates the DPoP proof JWT and returns the JWK thumbprint (jkt).
//
//   - accessToken: the raw access token value; used to verify the ath claim (RFC 9449 §4.2).
//   - checkJTI: called with the jti; must return true if the jti was already seen (replay).
//     Pass nil to skip anti-replay enforcement.
func VerifyProof(
	proof, expectedMethod, expectedURL, accessToken string,
	now time.Time,
	checkJTI func(string) bool,
) (string, error) {
	jwtToken, err := jwt.ParseSigned(
		proof,
		[]jose.SignatureAlgorithm{
			jose.ES256, jose.ES384, jose.ES512,
			jose.PS256, jose.PS384, jose.PS512,
			jose.RS256, jose.RS384, jose.RS512,
		},
	)
	if err != nil {
		return "", err
	}

	if len(jwtToken.Headers) == 0 || jwtToken.Headers[0].JSONWebKey == nil {
		return "", errors.New("dpop proof missing jwk header")
	}

	header := jwtToken.Headers[0]

	typ, _ := header.ExtraHeaders[jose.HeaderType].(string)
	if !strings.EqualFold(typ, "dpop+jwt") {
		return "", errors.New("dpop proof missing or invalid typ header")
	}

	jwk := header.JSONWebKey
	if !jwk.IsPublic() {
		return "", errors.New("dpop jwk must be public")
	}

	var claims proofClaims
	if err := jwtToken.Claims(jwk.Key, &claims); err != nil {
		return "", err
	}

	if !strings.EqualFold(claims.Htm, expectedMethod) {
		return "", errors.New("dpop htm mismatch")
	}

	if err := matchHTU(claims.Htu, expectedURL); err != nil {
		return "", err
	}

	if claims.Iat == 0 {
		return "", errors.New("dpop iat missing")
	}
	issuedAt := time.Unix(claims.Iat, 0)
	if issuedAt.After(now.Add(maxClockSkew)) {
		return "", errors.New("dpop iat in the future")
	}
	if now.Sub(issuedAt) > maxProofAge {
		return "", errors.New("dpop proof too old")
	}

	if claims.Jti == "" {
		return "", errors.New("dpop jti missing")
	}
	if checkJTI != nil && checkJTI(claims.Jti) {
		return "", errors.New("dpop jti replayed")
	}

	h := sha256.Sum256([]byte(accessToken))
	expectedAth := base64.RawURLEncoding.EncodeToString(h[:])
	if claims.Ath == "" {
		return "", errors.New("dpop ath missing")
	}
	if claims.Ath != expectedAth {
		return "", errors.New("dpop ath mismatch")
	}

	thumbprint, err := jwk.Thumbprint(crypto.SHA256)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(thumbprint), nil
}

// matchHTU compares the htu claim against the expected URL per RFC 3986:
// scheme and host are case-insensitive; path and query are case-sensitive.
func matchHTU(htu, expectedURL string) error {
	h, err := url.Parse(htu)
	if err != nil || !h.IsAbs() {
		return errors.New("invalid dpop htu")
	}
	e, err := url.Parse(expectedURL)
	if err != nil || !e.IsAbs() {
		return errors.New("invalid expected htu")
	}
	if !strings.EqualFold(h.Scheme, e.Scheme) ||
		!strings.EqualFold(h.Host, e.Host) ||
		h.Path != e.Path ||
		h.RawQuery != e.RawQuery {
		return errors.New("dpop htu mismatch")
	}
	return nil
}
