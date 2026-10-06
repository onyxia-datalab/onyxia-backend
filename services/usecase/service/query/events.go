package query

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/onyxia-datalab/onyxia-backend/internal/usercontext"
	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
)

// StreamSettings bounds the event streams.
type StreamSettings struct {
	// MaxDuration ends a stream whose service is still changing (done with
	// reason timeout); the client reopens it to keep following.
	MaxDuration time.Duration
	// GhostGrace is how long a Ghost is waited for before ending the stream:
	// the release of a starting install appears only a moment after its
	// record, so a fresh service is briefly a Ghost.
	GhostGrace time.Duration
}

// DefaultStreamSettings are the settings of the API.
var DefaultStreamSettings = StreamSettings{MaxDuration: 15 * time.Minute, GhostGrace: 30 * time.Second}

// Streamer follows services: it implements ports.ServiceFollower on top of a
// cluster watch, reading the state back through the same path as GetService
// on every change.
type Streamer struct {
	reader   *Reader
	quotas   ports.ProjectQuotaReader
	watcher  ports.ClusterWatcher
	settings StreamSettings
	// shutdown ends every stream once closed, so that a graceful shutdown
	// doesn't wait for them; clients then reconnect.
	shutdown <-chan struct{}
}

var _ ports.ServiceFollower = (*Streamer)(nil)

func NewStreamer(
	lifetime context.Context,
	reader *Reader,
	quotas ports.ProjectQuotaReader,
	watcher ports.ClusterWatcher,
	settings StreamSettings,
) *Streamer {
	return &Streamer{
		reader:   reader,
		quotas:   quotas,
		watcher:  watcher,
		settings: settings,
		shutdown: lifetime.Done(),
	}
}

func (s *Streamer) Follow(
	ctx context.Context,
	user usercontext.User,
	namespace, releaseID string,
) (<-chan domain.ServiceEvent, error) {
	if err := s.reader.namespaces.Check(user, namespace); err != nil {
		return nil, err
	}
	rec, err := s.reader.records.GetServiceRecord(ctx, namespace, releaseID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("read service record: %w", err)
	}
	if !s.reader.namespaces.CanAccessService(user.Username, namespace, rec.Owner, rec.Share) {
		return nil, domain.ErrNotFound
	}

	streamCtx, cancel := context.WithTimeout(ctx, s.settings.MaxDuration)
	go func() {
		select {
		case <-s.shutdown:
			cancel()
		case <-streamCtx.Done():
		}
	}()
	// Watch before reading the state: a change made in between is then
	// notified rather than missed.
	changes, err := s.watcher.Watch(streamCtx, namespace, releaseID)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("watch service: %w", err)
	}

	out := make(chan domain.ServiceEvent)
	st := &stream{Streamer: s, namespace: namespace, rec: rec, out: out}
	go func() {
		defer close(out)
		defer cancel()
		if err := st.run(streamCtx, ctx, changes); err != nil {
			slog.ErrorContext(ctx, "service event stream failed",
				slog.String("release", releaseID), slog.Any("error", err))
		}
	}()
	return out, nil
}

// stream is one open stream. Its methods take the stream's context: the
// client's request context, bounded by MaxDuration and the shutdown.
type stream struct {
	*Streamer
	namespace string
	out       chan<- domain.ServiceEvent
	rec       ports.ServiceRecord // the last record read
	deleted   bool                // the record is gone
	gone      bool                // deleted, and no pod left

	service domain.Service // the last status sent
	quota   domain.ProjectQuota
}

// run follows the service until the stream ends. clientCtx is the client's
// request context, which outlives ctx when the stream times out.
func (st *stream) run(ctx, clientCtx context.Context, changes <-chan ports.ClusterChange) error {
	if err := st.refreshService(ctx, true); err != nil {
		return err
	}
	if err := st.refreshQuota(ctx, true); err != nil {
		return err
	}
	if end, ok := st.ended(false); ok {
		st.send(ctx, domain.ServiceEvent{Kind: domain.ServiceEventDone, End: end})
		return nil
	}

	ghostGrace := time.NewTimer(st.settings.GhostGrace)
	defer ghostGrace.Stop()
	ghostGraceOver := false

	for {
		select {
		case <-ctx.Done():
			st.endOnTimeout(ctx, clientCtx)
			return nil
		case <-ghostGrace.C:
			ghostGraceOver = true
		case change, ok := <-changes:
			if !ok {
				// The watch ended: stop without done, the client reconnects.
				return nil
			}
			if err := st.apply(ctx, append([]ports.ClusterChange{change}, drain(changes)...)); err != nil {
				return err
			}
		}
		if end, ok := st.ended(ghostGraceOver); ok {
			st.send(ctx, domain.ServiceEvent{Kind: domain.ServiceEventDone, End: end})
			return nil
		}
	}
}

// drain takes the changes already waiting, so that a burst (a rollout
// updates its pods many times) is read back once.
func drain(changes <-chan ports.ClusterChange) []ports.ClusterChange {
	var batch []ports.ClusterChange
	for {
		select {
		case c, ok := <-changes:
			if !ok {
				return batch
			}
			batch = append(batch, c)
		default:
			return batch
		}
	}
}

// apply sends the progress of a batch of changes, then reads back the
// state they affect.
func (st *stream) apply(ctx context.Context, batch []ports.ClusterChange) error {
	serviceChanged, quotaChanged := false, false
	for _, c := range batch {
		switch c.Kind {
		case ports.ClusterChangeQuota:
			quotaChanged = true
		case ports.ClusterChangeProgress:
			// A progress step often goes with a state change (e.g. a pod
			// creation refused by the quota).
			serviceChanged = true
			if c.Progress != nil {
				st.send(ctx, domain.ServiceEvent{Kind: domain.ServiceEventProgress, Progress: c.Progress})
			}
		default:
			serviceChanged = true
		}
	}
	if serviceChanged {
		if err := st.refreshService(ctx, false); err != nil {
			return err
		}
	}
	if quotaChanged {
		return st.refreshQuota(ctx, false)
	}
	return nil
}

// refreshService reads the service back and sends its status if it changed.
// Once the record is gone (deleted), the last record read still describes
// the service while its pods shut down.
func (st *stream) refreshService(ctx context.Context, first bool) error {
	if !st.deleted {
		rec, err := st.reader.records.GetServiceRecord(ctx, st.namespace, st.rec.ReleaseID)
		switch {
		case errors.Is(err, domain.ErrNotFound):
			st.deleted = true
		case err != nil:
			return fmt.Errorf("read service record: %w", err)
		default:
			st.rec = rec
		}
	}

	svc, err := st.reader.readService(ctx, st.namespace, st.rec)
	if err != nil {
		return err
	}
	if st.deleted && svc.Status == domain.ServiceStatusGhost {
		// Deleted and no pod left: there is nothing more to show.
		st.gone = true
		return nil
	}
	if first || !reflect.DeepEqual(svc, st.service) {
		st.service = svc
		st.send(ctx, domain.ServiceEvent{Kind: domain.ServiceEventStatus, Service: &svc})
	}
	return nil
}

// refreshQuota reads the project's quota back and sends it if it changed.
// A project without quota gets no quota event.
func (st *stream) refreshQuota(ctx context.Context, first bool) error {
	quota, err := st.quotas.ReadProjectQuota(ctx, st.namespace)
	if err != nil {
		return fmt.Errorf("get project quota: %w", err)
	}
	if len(quota.Resources) == 0 && len(st.quota.Resources) == 0 {
		return nil
	}
	if first || !reflect.DeepEqual(quota, st.quota) {
		st.quota = quota
		st.send(ctx, domain.ServiceEvent{Kind: domain.ServiceEventQuota, Quota: &quota})
	}
	return nil
}

// ended tells whether the stream is over, and why.
func (st *stream) ended(ghostGraceOver bool) (domain.StreamEndReason, bool) {
	if st.deleted {
		return domain.StreamEndDeleted, st.gone
	}
	switch st.service.Status {
	case domain.ServiceStatusRunning, domain.ServiceStatusSuspended:
		return domain.StreamEndStable, true
	case domain.ServiceStatusGhost:
		return domain.StreamEndStable, ghostGraceOver
	case domain.ServiceStatusError:
		// Other errors may resolve by themselves (a crash loop, a quota).
		return domain.StreamEndStable,
			st.service.Error != nil && st.service.Error.Reason == domain.ServiceErrorReasonReleaseFailed
	default:
		return "", false
	}
}

// endOnTimeout sends done when the stream reached its maximum duration
// while the client is still there. A client gone or a shutdown gets nothing.
func (st *stream) endOnTimeout(ctx, clientCtx context.Context) {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) && clientCtx.Err() == nil && !st.shuttingDown() {
		st.send(clientCtx, domain.ServiceEvent{Kind: domain.ServiceEventDone, End: domain.StreamEndTimeout})
	}
}

func (s *Streamer) shuttingDown() bool {
	select {
	case <-s.shutdown:
		return true
	default:
		return false
	}
}

// send delivers an event unless ctx is done first.
func (st *stream) send(ctx context.Context, ev domain.ServiceEvent) {
	select {
	case st.out <- ev:
	case <-ctx.Done():
	}
}
