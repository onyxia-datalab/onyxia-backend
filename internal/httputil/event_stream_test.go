package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlushEventStreams(t *testing.T) {
	tests := []struct {
		contentType string
		wantFlushed bool
	}{
		{"text/event-stream", true},
		{"text/event-stream; charset=utf-8", true},
		{"application/json", false},
	}
	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h := FlushEventStreams(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				_, _ = w.Write([]byte("data"))
			}))

			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

			assert.Equal(t, tt.wantFlushed, rec.Flushed)
			assert.Equal(t, "data", rec.Body.String())
		})
	}
}
