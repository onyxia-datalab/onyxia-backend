package httputil

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func applyProxyHeaders(r *http.Request) *http.Request {
	var captured *http.Request
	handler := ProxyHeaders(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = r
	}))
	handler.ServeHTTP(httptest.NewRecorder(), r)
	return captured
}

func TestProxyHeaders_PlainHTTP(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	r := applyProxyHeaders(req)
	assert.Equal(t, "http", r.URL.Scheme)
	assert.Equal(t, "example.com", r.URL.Host)
}

func TestProxyHeaders_TLS(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	req.TLS = &tls.ConnectionState{}
	r := applyProxyHeaders(req)
	assert.Equal(t, "https", r.URL.Scheme)
}

func TestProxyHeaders_XForwardedProto(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	r := applyProxyHeaders(req)
	assert.Equal(t, "https", r.URL.Scheme)
}

func TestProxyHeaders_XForwardedProto_TakesPrecedenceOverTLS(t *testing.T) {
	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	req.TLS = &tls.ConnectionState{}
	req.Header.Set("X-Forwarded-Proto", "http")
	r := applyProxyHeaders(req)
	assert.Equal(t, "http", r.URL.Scheme)
}
