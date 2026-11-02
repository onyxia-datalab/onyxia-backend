package query

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/namespace"
	"github.com/onyxia-datalab/onyxia-backend/services/usecase/service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCluster is a mutable cluster state behind the gateways the Streamer
// reads, and the watch that notifies its changes.
type fakeCluster struct {
	mu           sync.Mutex
	record       *ports.ServiceRecord
	release      ports.ReleaseState
	pods         []ports.PodInfo
	quotaFailure string
	quota        domain.ProjectQuota

	changes chan ports.ClusterChange
	watched chan struct{} // closed once Watch is called
}

func newFakeCluster(release ports.ReleaseState, pods ...ports.PodInfo) *fakeCluster {
	rec := record(testRelease, testUsername, false)
	return &fakeCluster{
		record:  &rec,
		release: release,
		pods:    pods,
		changes: make(chan ports.ClusterChange, 16),
		watched: make(chan struct{}),
	}
}

// update changes the state, then notifies the change.
func (c *fakeCluster) update(kind ports.ClusterChangeKind, f func(c *fakeCluster)) {
	c.mu.Lock()
	f(c)
	c.mu.Unlock()
	c.changes <- ports.ClusterChange{Kind: kind}
}

type fakeRecords struct {
	*mocks.MockServiceRecordGateway
	c *fakeCluster
}

func (f fakeRecords) GetServiceRecord(context.Context, string, string) (ports.ServiceRecord, error) {
	f.c.mu.Lock()
	defer f.c.mu.Unlock()
	if f.c.record == nil {
		return ports.ServiceRecord{}, domain.ErrNotFound
	}
	return *f.c.record, nil
}

type fakeReleases struct {
	*mocks.MockReleaseGateway
	c *fakeCluster
}

func (f fakeReleases) GetReleaseState(context.Context, string, string) (ports.ReleaseState, error) {
	f.c.mu.Lock()
	defer f.c.mu.Unlock()
	return f.c.release, nil
}

type fakeWorkloads struct {
	*mocks.MockWorkloadStateGateway
	c *fakeCluster
}

func (f fakeWorkloads) GetPodsForRelease(context.Context, string, string) ([]ports.PodInfo, error) {
	f.c.mu.Lock()
	defer f.c.mu.Unlock()
	return append([]ports.PodInfo(nil), f.c.pods...), nil
}

func (f fakeWorkloads) ListQuotaFailures(context.Context, string) (map[string]string, error) {
	f.c.mu.Lock()
	defer f.c.mu.Unlock()
	if f.c.quotaFailure == "" {
		return map[string]string{}, nil
	}
	return map[string]string{testRelease: f.c.quotaFailure}, nil
}

func (c *fakeCluster) ReadProjectQuota(context.Context, string) (domain.ProjectQuota, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.quota, nil
}

func (c *fakeCluster) Watch(ctx context.Context, _, _ string) (<-chan ports.ClusterChange, error) {
	close(c.watched)
	out := make(chan ports.ClusterChange)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case change, ok := <-c.changes:
				if !ok {
					return
				}
				select {
				case out <- change:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}

var testStreamSettings = StreamSettings{MaxDuration: 5 * time.Second, GhostGrace: 5 * time.Second}

func newTestStreamer(lifetime context.Context, c *fakeCluster, settings StreamSettings) *Streamer {
	reader := NewReader(
		fakeRecords{new(mocks.MockServiceRecordGateway), c},
		fakeReleases{new(mocks.MockReleaseGateway), c},
		fakeWorkloads{new(mocks.MockWorkloadStateGateway), c},
		namespace.NewAuthorizer("user-", "projet-"),
	)
	return NewStreamer(lifetime, reader, c, c, settings)
}

func openStream(t *testing.T, c *fakeCluster, settings StreamSettings) <-chan domain.ServiceEvent {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	events, err := newTestStreamer(context.Background(), c, settings).Follow(ctx, testCaller, testNamespace, testRelease)
	require.NoError(t, err)
	return events
}

// next returns the next event, failing if none comes or the stream ends.
func next(t *testing.T, events <-chan domain.ServiceEvent) domain.ServiceEvent {
	t.Helper()
	select {
	case ev, ok := <-events:
		require.True(t, ok, "stream closed")
		return ev
	case <-time.After(2 * time.Second):
		require.FailNow(t, "no event")
		return domain.ServiceEvent{}
	}
}

func requireStatus(t *testing.T, events <-chan domain.ServiceEvent, want domain.ServiceStatus) domain.Service {
	t.Helper()
	ev := next(t, events)
	require.Equal(t, domain.ServiceEventStatus, ev.Kind, "event %+v", ev)
	require.Equal(t, want, ev.Service.Status)
	return *ev.Service
}

func requireDone(t *testing.T, events <-chan domain.ServiceEvent, want domain.StreamEndReason) {
	t.Helper()
	ev := next(t, events)
	require.Equal(t, domain.ServiceEventDone, ev.Kind, "event %+v", ev)
	require.Equal(t, want, ev.End)
	requireClosed(t, events)
}

func requireClosed(t *testing.T, events <-chan domain.ServiceEvent) {
	t.Helper()
	select {
	case ev, ok := <-events:
		require.False(t, ok, "unexpected event %+v", ev)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "stream not closed")
	}
}

var (
	deployedRelease = ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusDeployed}
	readyPod        = ports.PodInfo{Name: "p", Ready: true}
	startingPod     = ports.PodInfo{Name: "p"}
)

func TestStream_StableAtOnce(t *testing.T) {
	events := openStream(t, newFakeCluster(deployedRelease, readyPod), testStreamSettings)

	requireStatus(t, events, domain.ServiceStatusRunning)
	requireDone(t, events, domain.StreamEndStable)
}

func TestStream_FollowsAnInstallUntilRunning(t *testing.T) {
	c := newFakeCluster(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusPending})
	events := openStream(t, c, testStreamSettings)
	requireStatus(t, events, domain.ServiceStatusDeploying)

	c.update(ports.ClusterChangeRelease, func(c *fakeCluster) { c.release = deployedRelease; c.pods = []ports.PodInfo{startingPod} })
	// Deploying again: not a change, nothing is sent.
	progress := domain.ServiceProgress{Step: domain.ServiceProgressPullingImage, Image: "jupyter"}
	c.changes <- ports.ClusterChange{Kind: ports.ClusterChangeProgress, Progress: &progress}
	ev := next(t, events)
	require.Equal(t, domain.ServiceEventProgress, ev.Kind)
	assert.Equal(t, progress, *ev.Progress)

	c.update(ports.ClusterChangePods, func(c *fakeCluster) { c.pods = []ports.PodInfo{readyPod} })
	requireStatus(t, events, domain.ServiceStatusRunning)
	requireDone(t, events, domain.StreamEndStable)
}

func TestStream_QuotaIsSentFirstThenOnChange(t *testing.T) {
	c := newFakeCluster(deployedRelease, startingPod)
	c.quota = domain.ProjectQuota{Resources: []domain.QuotaResource{{Name: "requests.memory", Hard: "10Gi", Used: "2Gi"}}}
	events := openStream(t, c, testStreamSettings)

	requireStatus(t, events, domain.ServiceStatusDeploying)
	ev := next(t, events)
	require.Equal(t, domain.ServiceEventQuota, ev.Kind)
	assert.Equal(t, "2Gi", ev.Quota.Resources[0].Used)

	c.update(ports.ClusterChangeQuota, func(c *fakeCluster) {}) // unchanged: nothing sent
	c.update(ports.ClusterChangeQuota, func(c *fakeCluster) {
		c.quota = domain.ProjectQuota{Resources: []domain.QuotaResource{{Name: "requests.memory", Hard: "10Gi", Used: "6Gi"}}}
	})
	ev = next(t, events)
	require.Equal(t, domain.ServiceEventQuota, ev.Kind)
	assert.Equal(t, "6Gi", ev.Quota.Resources[0].Used)
}

func TestStream_QuotaExceededStaysOpenUntilResolved(t *testing.T) {
	c := newFakeCluster(deployedRelease)
	c.quotaFailure = "exceeded quota: requests.memory"
	events := openStream(t, c, testStreamSettings)

	svc := requireStatus(t, events, domain.ServiceStatusError)
	assert.Equal(t, domain.ServiceErrorReasonQuotaExceeded, svc.Error.Reason)

	c.update(ports.ClusterChangePods, func(c *fakeCluster) { c.quotaFailure = ""; c.pods = []ports.PodInfo{readyPod} })
	requireStatus(t, events, domain.ServiceStatusRunning)
	requireDone(t, events, domain.StreamEndStable)
}

func TestStream_FailedReleaseEnds(t *testing.T) {
	events := openStream(t, newFakeCluster(ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusFailed}), testStreamSettings)

	requireStatus(t, events, domain.ServiceStatusError)
	requireDone(t, events, domain.StreamEndStable)
}

// A delete: the release goes first, then the record; the stream follows the
// pods shutting down and ends once none is left.
func TestStream_FollowsADeleteUntilThePodsAreGone(t *testing.T) {
	c := newFakeCluster(deployedRelease, startingPod)
	events := openStream(t, c, testStreamSettings)
	requireStatus(t, events, domain.ServiceStatusDeploying)

	c.update(ports.ClusterChangeRelease, func(c *fakeCluster) {
		c.release = ports.ReleaseState{}
		c.record = nil
		c.pods = []ports.PodInfo{{Name: "p", Terminating: true}}
	})
	requireStatus(t, events, domain.ServiceStatusTerminating)

	c.update(ports.ClusterChangePods, func(c *fakeCluster) { c.pods = nil })
	requireDone(t, events, domain.StreamEndDeleted)
}

func TestStream_SuspendUntilThePodsAreGone(t *testing.T) {
	c := newFakeCluster(deployedRelease, readyPod)
	c.release.Suspended = true
	events := openStream(t, c, testStreamSettings)
	requireStatus(t, events, domain.ServiceStatusSuspending)

	c.update(ports.ClusterChangePods, func(c *fakeCluster) { c.pods = nil })
	requireStatus(t, events, domain.ServiceStatusSuspended)
	requireDone(t, events, domain.StreamEndStable)
}

// The release of a starting install appears a moment after its record.
func TestStream_GhostIsWaitedForDuringTheGrace(t *testing.T) {
	c := newFakeCluster(ports.ReleaseState{})
	events := openStream(t, c, testStreamSettings)
	requireStatus(t, events, domain.ServiceStatusGhost)

	c.update(ports.ClusterChangeRelease, func(c *fakeCluster) {
		c.release = ports.ReleaseState{Exists: true, Status: ports.ReleaseStatusPending}
	})
	requireStatus(t, events, domain.ServiceStatusDeploying)
}

func TestStream_GhostEndsAfterTheGrace(t *testing.T) {
	settings := testStreamSettings
	settings.GhostGrace = 10 * time.Millisecond
	events := openStream(t, newFakeCluster(ports.ReleaseState{}), settings)

	requireStatus(t, events, domain.ServiceStatusGhost)
	requireDone(t, events, domain.StreamEndStable)
}

func TestStream_TimeoutEndsWithDone(t *testing.T) {
	settings := testStreamSettings
	settings.MaxDuration = 20 * time.Millisecond
	events := openStream(t, newFakeCluster(deployedRelease, startingPod), settings)

	requireStatus(t, events, domain.ServiceStatusDeploying)
	requireDone(t, events, domain.StreamEndTimeout)
}

// A broken watch ends the stream without done: the client reconnects.
func TestStream_WatchEndClosesWithoutDone(t *testing.T) {
	c := newFakeCluster(deployedRelease, startingPod)
	events := openStream(t, c, testStreamSettings)
	requireStatus(t, events, domain.ServiceStatusDeploying)

	close(c.changes)
	requireClosed(t, events)
}

// At shutdown the streams end without done, so that the shutdown doesn't
// wait for them and the clients reconnect to another instance.
func TestStream_ShutdownClosesWithoutDone(t *testing.T) {
	lifetime, shutdown := context.WithCancel(context.Background())
	c := newFakeCluster(deployedRelease, startingPod)
	events, err := newTestStreamer(lifetime, c, testStreamSettings).
		Follow(context.Background(), testCaller, testNamespace, testRelease)
	require.NoError(t, err)
	requireStatus(t, events, domain.ServiceStatusDeploying)

	shutdown()
	requireClosed(t, events)
}

func TestStream_ClientGoneClosesTheStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := newFakeCluster(deployedRelease, startingPod)
	events, err := newTestStreamer(context.Background(), c, testStreamSettings).
		Follow(ctx, testCaller, testNamespace, testRelease)
	require.NoError(t, err)
	requireStatus(t, events, domain.ServiceStatusDeploying)

	cancel()
	requireClosed(t, events)
}

func TestStream_Authorization(t *testing.T) {
	t.Run("foreign namespace", func(t *testing.T) {
		c := newFakeCluster(deployedRelease)
		_, err := newTestStreamer(context.Background(), c, testStreamSettings).
			Follow(context.Background(), testCaller, "user-bob", testRelease)
		assert.ErrorIs(t, err, domain.ErrForbidden)
	})
	t.Run("unshared service of another owner", func(t *testing.T) {
		c := newFakeCluster(deployedRelease)
		rec := record(testRelease, "bob", false)
		c.record = &rec
		_, err := newTestStreamer(context.Background(), c, testStreamSettings).
			Follow(context.Background(), testCaller, testNamespace, testRelease)
		assert.ErrorIs(t, err, domain.ErrNotFound)
		select {
		case <-c.watched:
			t.Fatal("the cluster was watched for a service the caller can't see")
		default:
		}
	})
	t.Run("missing service", func(t *testing.T) {
		c := newFakeCluster(deployedRelease)
		c.record = nil
		_, err := newTestStreamer(context.Background(), c, testStreamSettings).
			Follow(context.Background(), testCaller, testNamespace, testRelease)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})
}
