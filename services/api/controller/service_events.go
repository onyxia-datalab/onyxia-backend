package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	api "github.com/onyxia-datalab/onyxia-backend/services/api/oas"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

// keepAliveInterval is how often a comment line is written on an idle
// stream, so that proxies don't close it.
const keepAliveInterval = 15 * time.Second

type ServiceEventsController struct {
	events     ports.ServiceEvents
	quotas     ports.ProjectQuotaQuery
	userGetter usercontext.UserGetter
	keepAlive  time.Duration
}

func NewServiceEventsController(
	events ports.ServiceEvents,
	quotas ports.ProjectQuotaQuery,
	userGetter usercontext.UserGetter,
) *ServiceEventsController {
	return &ServiceEventsController{events: events, quotas: quotas, userGetter: userGetter, keepAlive: keepAliveInterval}
}

// WatchServiceEvents opens the stream first, so that authorization errors
// are still answered with an HTTP error, then writes its events as SSE
// frames, flushing each one so that it reaches the client right away.
// Once the response has started, a failure can only end the stream: nil is
// returned, so that ogen doesn't write an error into it.
func (c *ServiceEventsController) WatchServiceEvents(
	ctx context.Context,
	params api.WatchServiceEventsParams,
	w http.ResponseWriter,
) error {
	user, err := callerFrom(ctx, c.userGetter)
	if err != nil {
		return err
	}

	events, err := c.events.Open(ctx, user, params.XOnyxiaProject, params.ReleaseId)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) && !errors.Is(err, domain.ErrForbidden) {
			slog.ErrorContext(ctx, "open service event stream failed", slog.Any("error", err))
		}
		return err
	}

	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")
	header.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(w)
	// Send the headers right away: the first event may take a while.
	if err := rc.Flush(); err != nil {
		slog.ErrorContext(ctx, "flush service event stream failed", slog.Any("error", err))
		return nil
	}

	keepAlive := time.NewTicker(c.keepAlive)
	defer keepAlive.Stop()
	for {
		var frame []byte
		select {
		case <-ctx.Done():
			// The client is gone. Returning ends the request, whose context
			// also stops the stream.
			return nil
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			if frame, err = encodeFrame(ev); err != nil {
				slog.ErrorContext(ctx, "encode service event failed", slog.Any("error", err))
				return nil
			}
		case <-keepAlive.C:
			frame = []byte(": keep-alive\n\n")
		}
		if _, err := w.Write(frame); err != nil {
			return nil
		}
		if err := rc.Flush(); err != nil {
			return nil
		}
	}
}

// encodeFrame writes an event in the SSE format: "event: <name>\ndata:
// <JSON>\n\n". The JSON has no newline, so it fits on one data line.
func encodeFrame(ev domain.ServiceEvent) ([]byte, error) {
	var data any
	switch ev.Kind {
	case domain.ServiceEventStatus:
		data = toAPIService(*ev.Service)
	case domain.ServiceEventProgress:
		data = toAPIProgress(*ev.Progress)
	case domain.ServiceEventQuota:
		data = toAPIQuota(*ev.Quota)
	case domain.ServiceEventDone:
		data = streamEnd{Reason: string(ev.End)}
	default:
		return nil, fmt.Errorf("unknown service event %q", ev.Kind)
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal %s event: %w", ev.Kind, err)
	}
	return fmt.Appendf(nil, "event: %s\ndata: %s\n\n", ev.Kind, payload), nil
}

// serviceProgress and streamEnd are the payloads of the progress and done
// events (ServiceProgress and StreamEnd in the OpenAPI spec). Only documented
// there, ogen doesn't generate them.
type serviceProgress struct {
	Step    string    `json:"step"`
	Level   string    `json:"level"`
	PodName string    `json:"podName,omitempty"`
	Image   string    `json:"image,omitempty"`
	Message string    `json:"message,omitempty"`
	At      time.Time `json:"at"`
}

type streamEnd struct {
	Reason string `json:"reason"`
}

func toAPIProgress(p domain.ServiceProgress) serviceProgress {
	return serviceProgress{
		Step:    string(p.Step),
		Level:   string(p.Level),
		PodName: p.PodName,
		Image:   p.Image,
		Message: p.Message,
		At:      p.At.UTC(),
	}
}

func toAPIQuota(q domain.ProjectQuota) *api.ProjectQuota {
	resources := make([]api.QuotaResource, 0, len(q.Resources))
	for _, r := range q.Resources {
		resources = append(resources, api.QuotaResource{Name: r.Name, Hard: r.Hard, Used: r.Used})
	}
	return &api.ProjectQuota{Resources: resources}
}

func (c *ServiceEventsController) GetProjectQuota(
	ctx context.Context,
	params api.GetProjectQuotaParams,
) (api.GetProjectQuotaRes, error) {
	user, err := callerFrom(ctx, c.userGetter)
	if err != nil {
		return nil, err
	}

	quota, err := c.quotas.GetProjectQuota(ctx, user, params.XOnyxiaProject)
	if err != nil {
		if !errors.Is(err, domain.ErrForbidden) {
			slog.ErrorContext(ctx, "get project quota failed", slog.Any("error", err))
		}
		return nil, err
	}
	return toAPIQuota(quota), nil
}
