package httputil

import (
	"mime"
	"net/http"
)

// FlushEventStreams flushes the responses of Content-Type text/event-stream
// after each write, so that every event reaches the client as soon as it is
// written. Handlers that copy a stream to the response (as ogen does) never
// flush themselves; other responses are left buffered.
func FlushEventStreams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(&eventStreamWriter{ResponseWriter: w, rc: http.NewResponseController(w)}, r)
	})
}

type eventStreamWriter struct {
	http.ResponseWriter
	rc *http.ResponseController
}

func (w *eventStreamWriter) WriteHeader(status int) {
	w.ResponseWriter.WriteHeader(status)
	// Send the headers right away: the first event may take a while.
	w.flushEventStream()
}

func (w *eventStreamWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if err == nil {
		w.flushEventStream()
	}
	return n, err
}

func (w *eventStreamWriter) flushEventStream() {
	mediaType, _, err := mime.ParseMediaType(w.Header().Get("Content-Type"))
	if err == nil && mediaType == "text/event-stream" {
		_ = w.rc.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *eventStreamWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
