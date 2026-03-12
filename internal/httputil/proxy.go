package httputil

import (
	"net/http"
)

// ProxyHeaders fixes r.URL.Scheme and r.URL.Host from X-Forwarded-Proto / r.Host
// so that downstream handlers always see an absolute request URL.
// This middleware must be registered before any handler that needs the full URL (e.g. DPoP).
func ProxyHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			r.URL.Scheme = proto
		} else if r.TLS != nil {
			r.URL.Scheme = "https"
		} else {
			r.URL.Scheme = "http"
		}
		if r.URL.Host == "" {
			r.URL.Host = r.Host
		}
		next.ServeHTTP(w, r)
	})
}
