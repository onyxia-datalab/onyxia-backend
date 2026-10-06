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

// One list call serves any number of releases; unlabelled pods and pods that
// ran to completion are left out.
func TestListPodsByRelease(t *testing.T) {
	deleting := releasePod("jupyter-old", "jupyter", corev1.PodRunning)
	deleting.DeletionTimestamp = &metav1.Time{Time: testNow}
	deleting.Finalizers = []string{"test"} // the fake client refuses a deletion timestamp without one
	cs := k8sfake.NewClientset(
		releasePod("jupyter-new", "jupyter", corev1.PodRunning),
		deleting,
		releasePod("rstudio", "rstudio", corev1.PodPending),
		releasePod("rstudio-evicted", "rstudio", corev1.PodFailed),
		releasePod("init-job", "rstudio", corev1.PodSucceeded),
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "unlabelled", Namespace: "project"}},
	)
	gateway := NewWorkloadStateGtw(cs)

	pods, err := gateway.ListPodsByRelease(context.Background(), "project")

	require.NoError(t, err)
	assert.Equal(t, map[string][]ports.PodInfo{
		"jupyter": {{Name: "jupyter-new"}, {Name: "jupyter-old", Terminating: true}},
		"rstudio": {{Name: "rstudio"}},
	}, pods)

	var verbs []string
	for _, a := range cs.Actions() {
		verbs = append(verbs, a.GetVerb()+" "+a.GetResource().Resource)
	}
	assert.Equal(t, []string{"list pods"}, verbs)
}

func TestGetPodsForReleaseFiltersByHelmLabel(t *testing.T) {
	client := k8sfake.NewClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "matching",
				Namespace: "project",
				Labels:    map[string]string{labelHelmInstance: "jupyter"},
			},
			Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Ready: true}}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "other-release",
				Namespace: "project",
				Labels:    map[string]string{labelHelmInstance: "rstudio"},
			},
		},
		releasePod("finished", "jupyter", corev1.PodSucceeded),
	)
	gateway := NewWorkloadStateGtw(client)

	pods, err := gateway.GetPodsForRelease(context.Background(), "project", "jupyter")

	require.NoError(t, err)
	require.Len(t, pods, 1)
	assert.Equal(t, "matching", pods[0].Name)
	assert.True(t, pods[0].Ready)
}

var testNow = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func TestDerivePodInfo(t *testing.T) {
	tests := []struct {
		name string
		pod  corev1.Pod
		want ports.PodInfo
	}{
		{
			name: "pod without container status is not ready",
			pod:  podNamed("pending"),
			want: ports.PodInfo{Name: "pending"},
		},
		{
			name: "all containers are ready",
			pod: podWithStatuses("ready",
				corev1.ContainerStatus{Ready: true},
				corev1.ContainerStatus{Ready: true},
			),
			want: ports.PodInfo{Name: "ready", Ready: true},
		},
		{
			name: "running container not ready within grace period is starting",
			pod: podWithStatuses("starting", corev1.ContainerStatus{
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.NewTime(testNow.Add(-time.Minute)),
				}},
			}),
			want: ports.PodInfo{Name: "starting"},
		},
		{
			name: "running container not ready after grace period failed readiness",
			pod: podWithStatuses("not-ready", corev1.ContainerStatus{
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{
					StartedAt: metav1.NewTime(testNow.Add(-readinessGracePeriod - time.Second)),
				}},
			}),
			want: ports.PodInfo{Name: "not-ready", ErrorReason: domain.ServiceErrorReasonReadinessFailed},
		},
		{
			name: "unschedulable condition is reported",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "unschedulable"},
				Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{
					Type:    corev1.PodScheduled,
					Status:  corev1.ConditionFalse,
					Reason:  "Unschedulable",
					Message: "insufficient cpu",
				}}},
			},
			want: ports.PodInfo{
				Name:        "unschedulable",
				ErrorReason: domain.ServiceErrorReasonUnschedulable,
				Message:     "insufficient cpu",
			},
		},
		{
			name: "container error takes priority over unschedulable condition",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "image-pull"},
				Status: corev1.PodStatus{
					Conditions: []corev1.PodCondition{{
						Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable",
					}},
					ContainerStatuses: []corev1.ContainerStatus{{
						Image: "registry.invalid/image",
						State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
							Reason: "ImagePullBackOff", Message: "back-off pulling image",
						}},
					}},
				},
			},
			want: ports.PodInfo{
				Name:        "image-pull",
				ErrorReason: domain.ServiceErrorReasonImagePull,
				Image:       "registry.invalid/image",
				Message:     "back-off pulling image",
			},
		},
		{
			name: "crash loop has the highest priority",
			pod: podWithStatuses("crash-loop",
				corev1.ContainerStatus{
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
						Reason: "OOMKilled", ExitCode: 137,
					}},
				},
				corev1.ContainerStatus{
					RestartCount: 4,
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
						Reason: "CrashLoopBackOff", Message: "back-off restarting container",
					}},
				},
			),
			want: ports.PodInfo{
				Name:         "crash-loop",
				ErrorReason:  domain.ServiceErrorReasonCrashLoop,
				RestartCount: 4,
				Message:      "back-off restarting container",
			},
		},
		{
			name: "a pod being deleted stays terminating when it has an error",
			pod: func() corev1.Pod {
				pod := podWithStatuses("deleting", corev1.ContainerStatus{
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
				})
				pod.DeletionTimestamp = &metav1.Time{Time: testNow}
				return pod
			}(),
			want: ports.PodInfo{Name: "deleting", Terminating: true, ErrorReason: domain.ServiceErrorReasonCrashLoop},
		},
		{
			name: "last OOM termination is retained while container restarts",
			pod: podWithStatuses("oom", corev1.ContainerStatus{
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					Reason: "OOMKilled", ExitCode: 137,
				}},
			}),
			want: ports.PodInfo{
				Name:        "oom",
				ErrorReason: domain.ServiceErrorReasonOOMKilled,
				ExitCode:    137,
			},
		},
		{
			name: "last OOM termination is ignored once the container is ready again",
			pod: podWithStatuses("recovered", corev1.ContainerStatus{
				Ready: true,
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
					Reason: "OOMKilled", ExitCode: 137,
				}},
			}),
			want: ports.PodInfo{Name: "recovered", Ready: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, derivePodInfo(tt.pod, testNow))
		})
	}
}

func releasePod(name, release string, phase corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "project",
			Labels:    map[string]string{labelHelmInstance: release},
		},
		Status: corev1.PodStatus{Phase: phase},
	}
}

func podNamed(name string) corev1.Pod {
	return corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func podWithStatuses(name string, statuses ...corev1.ContainerStatus) corev1.Pod {
	pod := podNamed(name)
	pod.Status.ContainerStatuses = statuses
	return pod
}
