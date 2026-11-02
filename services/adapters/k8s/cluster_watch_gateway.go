package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sync"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// K8sClusterWatchGateway watches, for one release: its pods, its Helm
// release (the Secrets of Helm's storage), its Onyxia record, the
// namespace's ResourceQuotas and the Kubernetes events of its workloads.
type K8sClusterWatchGateway struct {
	client kubernetes.Interface
}

var _ ports.ClusterWatcher = (*K8sClusterWatchGateway)(nil)

func NewClusterWatchGtw(client kubernetes.Interface) *K8sClusterWatchGateway {
	return &K8sClusterWatchGateway{client: client}
}

// Labels Helm's Secret storage driver puts on the Secrets holding a release.
const (
	labelHelmOwner = "owner"
	labelHelmName  = "name"
)

type watchSource struct {
	kind  ports.ClusterChangeKind
	watch watch.Interface
}

func (g *K8sClusterWatchGateway) Watch(
	ctx context.Context,
	namespace, releaseID string,
) (<-chan ports.ClusterChange, error) {
	ctx, cancel := context.WithCancel(ctx)

	core := g.client.CoreV1()
	opens := []struct {
		kind ports.ClusterChangeKind
		open func() (watch.Interface, error)
	}{
		{ports.ClusterChangePods, func() (watch.Interface, error) {
			return core.Pods(namespace).Watch(ctx, metav1.ListOptions{
				LabelSelector: labelHelmInstance + "=" + releaseID,
			})
		}},
		{ports.ClusterChangeRelease, func() (watch.Interface, error) {
			return core.Secrets(namespace).Watch(ctx, metav1.ListOptions{
				LabelSelector: labelHelmOwner + "=helm," + labelHelmName + "=" + releaseID,
			})
		}},
		{ports.ClusterChangeRelease, func() (watch.Interface, error) {
			return core.Secrets(namespace).Watch(ctx, metav1.ListOptions{
				FieldSelector: "metadata.name=" + buildOnyxiaSecretName(releaseID),
			})
		}},
		{ports.ClusterChangeQuota, func() (watch.Interface, error) {
			return core.ResourceQuotas(namespace).Watch(ctx, metav1.ListOptions{})
		}},
		{ports.ClusterChangeProgress, func() (watch.Interface, error) {
			return core.Events(namespace).Watch(ctx, metav1.ListOptions{})
		}},
	}

	sources := make([]watchSource, 0, len(opens))
	for _, o := range opens {
		w, err := o.open()
		if err != nil {
			cancel()
			for _, s := range sources {
				s.watch.Stop()
			}
			return nil, fmt.Errorf("watch %s: %w", o.kind, err)
		}
		sources = append(sources, watchSource{kind: o.kind, watch: w})
	}

	out := make(chan ports.ClusterChange, 64)
	go g.run(ctx, cancel, sources, newOwnerCache(g.client, namespace, releaseID), out)
	return out, nil
}

// run forwards every source to out, and closes out once they have all
// ended. When one watch ends (the API server closes watches after a while),
// it stops them all: the caller sees the channel close and starts over from
// a fresh state rather than missing changes.
func (g *K8sClusterWatchGateway) run(
	ctx context.Context,
	cancel context.CancelFunc,
	sources []watchSource,
	owners *ownerCache,
	out chan<- ports.ClusterChange,
) {
	var wg sync.WaitGroup
	for _, s := range sources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer cancel()
			defer s.watch.Stop()
			g.forward(ctx, s, owners, out)
		}()
	}
	wg.Wait()
	close(out)
}

// forward translates the events of one watch into ClusterChanges until the
// watch ends or ctx is done.
func (g *K8sClusterWatchGateway) forward(
	ctx context.Context,
	s watchSource,
	owners *ownerCache,
	out chan<- ports.ClusterChange,
) {
	for {
		var ev watch.Event
		select {
		case <-ctx.Done():
			return
		case e, ok := <-s.watch.ResultChan():
			if !ok {
				return
			}
			ev = e
		}
		if ev.Type == watch.Error {
			slog.WarnContext(ctx, "cluster watch failed",
				slog.String("source", string(s.kind)),
				slog.Any("status", apierrors.FromObject(ev.Object)))
			return
		}

		change := ports.ClusterChange{Kind: s.kind}
		if s.kind == ports.ClusterChangeProgress {
			e, ok := ev.Object.(*corev1.Event)
			if !ok || ev.Type == watch.Deleted || !owners.owns(ctx, e.InvolvedObject) {
				continue
			}
			progress, ok := progressFromEvent(*e)
			if !ok {
				continue
			}
			change.Progress = &progress
		}

		select {
		case out <- change:
		case <-ctx.Done():
			return
		}
	}
}

// ownerCache tells whether an object belongs to the release, by reading its
// Helm instance label. Objects that no longer exist (e.g. the pods of an
// earlier rollout, or the previous pod of a StatefulSet, whose events linger)
// are not the release's.
type ownerCache struct {
	client    kubernetes.Interface
	namespace string
	releaseID string

	mu    sync.Mutex
	known map[corev1.ObjectReference]bool
}

func newOwnerCache(client kubernetes.Interface, namespace, releaseID string) *ownerCache {
	return &ownerCache{
		client:    client,
		namespace: namespace,
		releaseID: releaseID,
		known:     map[corev1.ObjectReference]bool{},
	}
}

func (c *ownerCache) owns(ctx context.Context, ref corev1.ObjectReference) bool {
	key := corev1.ObjectReference{Kind: ref.Kind, Name: ref.Name, UID: ref.UID}
	c.mu.Lock()
	owned, ok := c.known[key]
	c.mu.Unlock()
	if ok {
		return owned
	}

	obj, err := currentObject(ctx, c.client, c.namespace, key)
	if err != nil {
		// Not cached: a later event of the same object retries.
		return false
	}
	owned = obj != nil && obj.GetLabels()[labelHelmInstance] == c.releaseID

	c.mu.Lock()
	c.known[key] = owned
	c.mu.Unlock()
	return owned
}

// imageInMessage extracts the image from the kubelet's image events, e.g.
// `Pulling image "inseefrlab/onyxia-jupyter-python:py3.12"`.
var imageInMessage = regexp.MustCompile(`(?i)image "([^"]+)"`)

// progressFromEvent translates a Kubernetes event into a ServiceProgress.
// ok is false for an event not worth showing: an expected (Normal) event
// outside the known steps.
func progressFromEvent(e corev1.Event) (progress domain.ServiceProgress, ok bool) {
	progress = domain.ServiceProgress{
		Level:   domain.ProgressLevelInfo,
		Message: e.Message,
		At:      eventTime(e),
	}
	if e.InvolvedObject.Kind == kindPod {
		progress.PodName = e.InvolvedObject.Name
	}

	switch e.Reason {
	case "Scheduled":
		progress.Step = domain.ServiceProgressScheduled
	case "FailedScheduling":
		progress.Step = domain.ServiceProgressSchedulingFailed
	case "Pulling":
		progress.Step = domain.ServiceProgressPullingImage
	case "Pulled":
		progress.Step = domain.ServiceProgressImagePulled
	case "Started":
		progress.Step = domain.ServiceProgressStarted
	case "Unhealthy":
		progress.Step = domain.ServiceProgressProbeFailed
	case reasonFailedCreate:
		progress.Step = domain.ServiceProgressCreationFailed
	default:
		if e.Type != corev1.EventTypeWarning {
			return domain.ServiceProgress{}, false
		}
		progress.Step = domain.ServiceProgressOther
	}

	if e.Type == corev1.EventTypeWarning {
		progress.Level = domain.ProgressLevelWarning
	}
	if m := imageInMessage.FindStringSubmatch(e.Message); m != nil &&
		(progress.Step == domain.ServiceProgressPullingImage || progress.Step == domain.ServiceProgressImagePulled) {
		progress.Image = m[1]
	}
	return progress, true
}
