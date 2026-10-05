package oidc

import (
	"crypto/tls"
	"net/http"
)

// insecureHTTPClient returns an HTTP client that does not verify the IdP's TLS
// certificate. It is only used for discovery and JWKS requests when
// oidc.skipTLSVerify is set.
func insecureHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // explicit opt-in via oidc.skipTLSVerify
	}
	return &http.Client{Transport: transport}
}
