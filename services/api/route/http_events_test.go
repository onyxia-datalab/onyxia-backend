package route

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readFrame reads one SSE frame (up to the blank line), failing if it
// doesn't come: a frame that stays buffered server-side never arrives.
func readFrame(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	frame := make(chan string, 1)
	go func() {
		var b strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				frame <- "EOF:" + b.String()
				return
			}
			if line == "\n" {
				frame <- b.String()
				return
			}
			b.WriteString(line)
		}
	}()
	select {
	case f := <-frame:
		return f
	case <-time.After(2 * time.Second):
		require.FailNow(t, "no frame")
		return ""
	}
}

func TestHTTP_WatchServiceEvents_StreamsEachEventAsItComes(t *testing.T) {
	events := make(chan domain.ServiceEvent)
	ts := newEventsTestServer(t, &stubLifecycle{}, &stubQuery{}, &stubEvents{events: events}, &stubQuotas{})

	// Bounded: if the headers stayed buffered server-side, the request
	// would wait for them forever.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/services/my-release/events", nil)
	require.NoError(t, err)
	req.Header.Set("X-Test-User", "alice")
	req.Header.Set("X-Onyxia-Project", testNamespace)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "the response headers must be flushed before the first event")
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no", resp.Header.Get("X-Accel-Buffering"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	body := bufio.NewReader(resp.Body)

	events <- domain.ServiceEvent{Kind: domain.ServiceEventStatus, Service: &domain.Service{
		ReleaseID: "my-release", Owner: "alice", Status: domain.ServiceStatusDeploying,
	}}
	frame := readFrame(t, body)
	require.True(t, strings.HasPrefix(frame, "event: status\ndata: "), frame)
	var svc map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(frame), "event: status\ndata: ")), &svc))
	assert.Equal(t, "Deploying", svc["status"])

	at := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	events <- domain.ServiceEvent{Kind: domain.ServiceEventProgress, Progress: &domain.ServiceProgress{
		Step: domain.ServiceProgressPullingImage, Level: domain.ProgressLevelInfo, Image: "jupyter", At: at,
	}}
	assert.Equal(t,
		"event: progress\ndata: {\"step\":\"pulling_image\",\"level\":\"info\",\"image\":\"jupyter\",\"at\":\"2026-10-06T10:00:00Z\"}\n",
		readFrame(t, body))

	events <- domain.ServiceEvent{Kind: domain.ServiceEventQuota, Quota: &domain.ProjectQuota{
		Resources: []domain.QuotaResource{{Name: "requests.memory", Hard: "10Gi", Used: "2Gi"}},
	}}
	assert.Equal(t,
		"event: quota\ndata: {\"resources\":[{\"name\":\"requests.memory\",\"hard\":\"10Gi\",\"used\":\"2Gi\"}]}\n",
		readFrame(t, body))

	events <- domain.ServiceEvent{Kind: domain.ServiceEventDone, End: domain.StreamEndStable}
	close(events)
	assert.Equal(t, "event: done\ndata: {\"reason\":\"stable\"}\n", readFrame(t, body))

	rest, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Empty(t, rest)
}

func TestHTTP_WatchServiceEvents_Errors(t *testing.T) {
	tests := []struct {
		name       string
		openErr    error
		wantStatus int
	}{
		{"not found maps to 404", domain.ErrNotFound, http.StatusNotFound},
		{"forbidden maps to 403", domain.ErrForbidden, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newEventsTestServer(t, &stubLifecycle{}, &stubQuery{}, &stubEvents{openErr: tt.openErr}, &stubQuotas{})
			resp := doRequest(t, ts, http.MethodGet, "/api/services/my-release/events", "alice", nil)
			require.Equal(t, tt.wantStatus, resp.StatusCode)
			assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
		})
	}

	t.Run("unauthenticated maps to 401", func(t *testing.T) {
		ts := newEventsTestServer(t, &stubLifecycle{}, &stubQuery{}, &stubEvents{}, &stubQuotas{})
		resp := doRequest(t, ts, http.MethodGet, "/api/services/my-release/events", "", nil)
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

func TestHTTP_GetProjectQuota(t *testing.T) {
	quotas := &stubQuotas{quota: domain.ProjectQuota{
		Resources: []domain.QuotaResource{{Name: "requests.cpu", Hard: "4", Used: "1500m"}},
	}}
	ts := newEventsTestServer(t, &stubLifecycle{}, &stubQuery{}, &stubEvents{}, quotas)

	resp := doRequest(t, ts, http.MethodGet, "/api/services/project/quota", "alice", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"resources":[{"name":"requests.cpu","hard":"4","used":"1500m"}]}`, string(body))

	t.Run("forbidden maps to 403", func(t *testing.T) {
		ts := newEventsTestServer(t, &stubLifecycle{}, &stubQuery{}, &stubEvents{}, &stubQuotas{err: domain.ErrForbidden})
		resp := doRequest(t, ts, http.MethodGet, "/api/services/project/quota", "alice", nil)
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("an empty quota is an empty list", func(t *testing.T) {
		ts := newEventsTestServer(t, &stubLifecycle{}, &stubQuery{}, &stubEvents{}, &stubQuotas{})
		resp := doRequest(t, ts, http.MethodGet, "/api/services/project/quota", "alice", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"resources":[]}`, string(body))
	})
}
