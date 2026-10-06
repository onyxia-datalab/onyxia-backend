package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestProgressFromEvent(t *testing.T) {
	at := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	podEvent := func(eventType, reason, message string) corev1.Event {
		return corev1.Event{
			InvolvedObject: corev1.ObjectReference{Kind: kindPod, Name: "jupyter-0"},
			Type:           eventType,
			Reason:         reason,
			Message:        message,
			LastTimestamp:  metav1.Time{Time: at},
		}
	}

	tests := []struct {
		name   string
		event  corev1.Event
		want   domain.ServiceProgress
		wantOK bool
	}{
		{
			name:   "image pull",
			event:  podEvent(corev1.EventTypeNormal, "Pulling", `Pulling image "inseefrlab/onyxia-jupyter-python:py3.12"`),
			want:   domain.ServiceProgress{Step: domain.ServiceProgressPullingImage, Image: "inseefrlab/onyxia-jupyter-python:py3.12"},
			wantOK: true,
		},
		{
			name:   "image already present",
			event:  podEvent(corev1.EventTypeNormal, "Pulled", `Container image "busybox" already present on machine`),
			want:   domain.ServiceProgress{Step: domain.ServiceProgressImagePulled, Image: "busybox"},
			wantOK: true,
		},
		{
			name:   "probe failure is a warning",
			event:  podEvent(corev1.EventTypeWarning, "Unhealthy", "Readiness probe failed"),
			want:   domain.ServiceProgress{Step: domain.ServiceProgressProbeFailed, Level: domain.ProgressLevelWarning},
			wantOK: true,
		},
		{
			name:   "unknown warning",
			event:  podEvent(corev1.EventTypeWarning, "FailedMount", "volume not found"),
			want:   domain.ServiceProgress{Step: domain.ServiceProgressOther, Level: domain.ProgressLevelWarning},
			wantOK: true,
		},
		{
			name:  "unknown normal event is skipped",
			event: podEvent(corev1.EventTypeNormal, "Created", "Created container"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := progressFromEvent(tt.event)
			require.Equal(t, tt.wantOK, ok)
			if !ok {
				return
			}
			if tt.want.Level == "" {
				tt.want.Level = domain.ProgressLevelInfo
			}
			tt.want.PodName = "jupyter-0"
			tt.want.Message = tt.event.Message
			tt.want.At = at
			assert.Equal(t, tt.want, got)
		})
	}
}

// The watch notifies the release's pod changes and translates the events of
// its own objects only.
func TestClusterWatch(t *testing.T) {
	client := k8sfake.NewClientset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changes, err := NewClusterWatchGtw(client).Watch(ctx, "project", "jupyter")
	require.NoError(t, err)

	pod := releasePod("jupyter-0", "jupyter", corev1.PodPending)
	_, err = client.CoreV1().Pods("project").Create(ctx, pod, metav1.CreateOptions{})
	require.NoError(t, err)
	assert.Equal(t, ports.ClusterChange{Kind: ports.ClusterChangePods}, nextChange(t, changes))

	// An event of another release's pod, then one of the release's pod.
	_, err = client.CoreV1().Pods("project").Create(ctx, releasePod("rstudio-0", "rstudio", corev1.PodPending), metav1.CreateOptions{})
	require.NoError(t, err)
	for _, e := range []*corev1.Event{
		{
			ObjectMeta:     metav1.ObjectMeta{Name: "other", Namespace: "project"},
			InvolvedObject: corev1.ObjectReference{Kind: kindPod, Name: "rstudio-0"},
			Type:           corev1.EventTypeNormal,
			Reason:         "Scheduled",
		},
		{
			ObjectMeta:     metav1.ObjectMeta{Name: "mine", Namespace: "project"},
			InvolvedObject: corev1.ObjectReference{Kind: kindPod, Name: "jupyter-0"},
			Type:           corev1.EventTypeNormal,
			Reason:         "Scheduled",
		},
	} {
		_, err = client.CoreV1().Events("project").Create(ctx, e, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// The fake client doesn't filter watches by label, so the other pod's
	// creation shows up as a pods change; its event, though, is dropped.
	for {
		change := nextChange(t, changes)
		if change.Kind != ports.ClusterChangeProgress {
			continue
		}
		require.NotNil(t, change.Progress)
		assert.Equal(t, domain.ServiceProgressScheduled, change.Progress.Step)
		assert.Equal(t, "jupyter-0", change.Progress.PodName)
		break
	}

	cancel()
	for range changes {
	}
}

func nextChange(t *testing.T, changes <-chan ports.ClusterChange) ports.ClusterChange {
	t.Helper()
	select {
	case c, ok := <-changes:
		require.True(t, ok, "watch closed")
		return c
	case <-time.After(2 * time.Second):
		require.FailNow(t, "no change")
		return ports.ClusterChange{}
	}
}
