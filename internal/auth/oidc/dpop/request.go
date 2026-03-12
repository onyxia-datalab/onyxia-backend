package dpop

import (
	"net/http"
	"strings"
)

// FindAuthorization extracts the token value from the Authorization header
// for the given scheme prefix (e.g. "DPoP", "Bearer").
func FindAuthorization(h http.Header, prefix string) (string, bool) {
	v, ok := h["Authorization"]
	if !ok {
		return "", false
	}
	for _, vv := range v {
		scheme, value, ok := strings.Cut(vv, " ")
		if !ok || !strings.EqualFold(scheme, prefix) {
			continue
		}
		return value, true
	}
	return "", false
}

// AbsoluteRequestURL returns the absolute URL of the request.
// It relies on the ProxyHeaders middleware having already set r.URL.Scheme and r.URL.Host.
// Fragment is stripped per RFC 9449 §4.2 (htu must not include fragment).
func AbsoluteRequestURL(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	u := *r.URL
	u.Fragment = ""
	return u.String()
}
