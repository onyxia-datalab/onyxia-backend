package server

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type serveResult struct{ err error }

// startServer serves handler on a random port and returns its base URL, the
// cancel function that triggers the graceful shutdown, and a channel
// receiving serve's result.
func startServer(
	t *testing.T,
	shutdownTimeout time.Duration,
	handler http.Handler,
	drains ...DrainFunc,
) (string, context.CancelFunc, <-chan serveResult) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	done := make(chan serveResult, 1)
	go func() { done <- serveResult{serve(ctx, ln, shutdownTimeout, handler, drains...)} }()

	return "http://" + ln.Addr().String(), cancel, done
}

func waitResult(t *testing.T, done <-chan serveResult) error {
	t.Helper()
	select {
	case r := <-done:
		return r.err
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
		return nil
	}
}

func TestHandler_Healthz(t *testing.T) {
	url, cancel, done := startServer(t, time.Second, Handler(http.NotFoundHandler(), nil))

	resp, err := http.Get(url + "/healthz")
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	cancel()
	assert.NoError(t, waitResult(t, done))
}

func TestHandler_CORSPreflightAllowsServiceMethods(t *testing.T) {
	handler := Handler(http.NotFoundHandler(), []string{"https://onyxia.example.org"})
	url, _, _ := startServer(t, time.Second, handler)

	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		req, err := http.NewRequest(http.MethodOptions, url+"/api/services/x", nil)
		require.NoError(t, err)
		req.Header.Set("Origin", "https://onyxia.example.org")
		req.Header.Set("Access-Control-Request-Method", method)
		req.Header.Set("Access-Control-Request-Headers", "X-Onyxia-Project")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = resp.Body.Close()

		assert.Equal(t, method, resp.Header.Get("Access-Control-Allow-Methods"), method)
		assert.Equal(t, "X-Onyxia-Project", resp.Header.Get("Access-Control-Allow-Headers"), method)
	}
}

func TestServe_InFlightRequestCompletesBeforeDrains(t *testing.T) {
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	var (
		mu    sync.Mutex
		order []string
	)
	record := func(step string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, step)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(requestStarted)
		<-releaseRequest
		_, _ = io.WriteString(w, "done")
		record("request")
	})
	drain := func(context.Context) error {
		record("drain")
		return nil
	}
	url, cancel, done := startServer(t, 5*time.Second, handler, drain)

	respCh := make(chan string, 1)
	go func() {
		resp, err := http.Get(url)
		if err != nil {
			respCh <- "error: " + err.Error()
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		respCh <- string(body)
	}()

	<-requestStarted
	cancel() // shutdown begins while the request is in flight
	time.Sleep(50 * time.Millisecond)
	close(releaseRequest)

	assert.Equal(t, "done", <-respCh)
	require.NoError(t, waitResult(t, done))
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"request", "drain"}, order)
}

func TestServe_DrainTimeoutIsReported(t *testing.T) {
	blockingDrain := func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}
	_, cancel, done := startServer(t, 50*time.Millisecond, http.NotFoundHandler(), blockingDrain)

	cancel()

	assert.True(t, errors.Is(waitResult(t, done), context.DeadlineExceeded))
}
