package dpop

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- FindAuthorization ---

func TestFindAuthorization_Missing(t *testing.T) {
	_, ok := FindAuthorization(http.Header{}, "Bearer")
	assert.False(t, ok)
}

func TestFindAuthorization_WrongScheme(t *testing.T) {
	h := http.Header{"Authorization": {"Bearer mytoken"}}
	_, ok := FindAuthorization(h, "DPoP")
	assert.False(t, ok)
}

func TestFindAuthorization_Bearer(t *testing.T) {
	h := http.Header{"Authorization": {"Bearer mytoken"}}
	token, ok := FindAuthorization(h, "Bearer")
	assert.True(t, ok)
	assert.Equal(t, "mytoken", token)
}

func TestFindAuthorization_DPoP(t *testing.T) {
	h := http.Header{"Authorization": {"DPoP dpoptoken"}}
	token, ok := FindAuthorization(h, "DPoP")
	assert.True(t, ok)
	assert.Equal(t, "dpoptoken", token)
}

func TestFindAuthorization_CaseInsensitive(t *testing.T) {
	h := http.Header{"Authorization": {"BEARER mytoken"}}
	token, ok := FindAuthorization(h, "bearer")
	assert.True(t, ok)
	assert.Equal(t, "mytoken", token)
}

func TestFindAuthorization_MultipleValues_PicksMatchingScheme(t *testing.T) {
	h := http.Header{"Authorization": {"Bearer first", "DPoP second"}}
	token, ok := FindAuthorization(h, "DPoP")
	assert.True(t, ok)
	assert.Equal(t, "second", token)
}

// --- AbsoluteRequestURL ---
// These tests assume ProxyHeaders middleware has already set r.URL.Scheme and r.URL.Host.

func TestAbsoluteRequestURL_NilRequest(t *testing.T) {
	assert.Equal(t, "", AbsoluteRequestURL(nil))
}

func TestAbsoluteRequestURL_NilURL(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	req.URL = nil
	assert.Equal(t, "", AbsoluteRequestURL(req))
}

func TestAbsoluteRequestURL_UsesSchemeAndHost(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/api/resource?q=1", nil)
	// simulate ProxyHeaders having set these
	req.URL.Scheme = "https"
	req.URL.Host = "example.com"
	assert.Equal(t, "https://example.com/api/resource?q=1", AbsoluteRequestURL(req))
}

func TestAbsoluteRequestURL_FragmentStripped(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	req.URL.Fragment = "section"
	assert.Equal(t, "http://example.com/api", AbsoluteRequestURL(req))
}
