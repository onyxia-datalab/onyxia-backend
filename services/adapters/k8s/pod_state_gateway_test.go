package k8s

import (
	"context"
	"testing"

	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestGetControllerReadiness(t *testing.T) {
	tests := []struct {
		name      string
		objects   []runtime.Object
		resources []ports.ManifestResource
		wantReady bool
		wantError bool
	}{
		{
			name: "all supported controllers are ready",
			objects: []runtime.Object{
				deployment("web", int32Pointer(2), 2),
				statefulSet("database", nil, 1),
			},
			resources: []ports.ManifestResource{
				{Kind: "Deployment", Name: "web"},
				{Kind: "Service", Name: "web"},
				{Kind: "StatefulSet", Name: "database"},
			},
			wantReady: true,
		},
		{
			name: "deployment is not ready",
			objects: []runtime.Object{
				deployment("web", int32Pointer(2), 1),
			},
			resources: []ports.ManifestResource{{Kind: "Deployment", Name: "web"}},
			wantReady: false,
		},
		{
			name: "stateful set is not ready",
			objects: []runtime.Object{
				statefulSet("database", int32Pointer(1), 0),
			},
			resources: []ports.ManifestResource{{Kind: "StatefulSet", Name: "database"}},
			wantReady: false,
		},
		{
			name:      "unknown resources are ignored",
			resources: []ports.ManifestResource{{Kind: "Service", Name: "web"}},
			wantReady: true,
		},
		{
			name:      "missing controller returns an error",
			resources: []ports.ManifestResource{{Kind: "Deployment", Name: "missing"}},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := NewWorkloadStateGtw(k8sfake.NewClientset(tt.objects...))

			ready, err := gateway.GetControllerReadiness(
				context.Background(),
				"project",
				tt.resources,
			)

			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantReady, ready)
		})
	}
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
	)
	gateway := NewWorkloadStateGtw(client)

	pods, err := gateway.GetPodsForRelease(context.Background(), "project", "jupyter")

	require.NoError(t, err)
	require.Len(t, pods, 1)
	assert.Equal(t, "matching", pods[0].Name)
	assert.True(t, pods[0].Ready)
}

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
			name: "running container failed readiness",
			pod: podWithStatuses("not-ready", corev1.ContainerStatus{
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}),
			want: ports.PodInfo{Name: "not-ready", ErrorReason: ports.PodErrorReasonReadinessFailed},
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
				ErrorReason: ports.PodErrorReasonUnschedulable,
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
				ErrorReason: ports.PodErrorReasonImagePull,
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
				ErrorReason:  ports.PodErrorReasonCrashLoop,
				RestartCount: 4,
				Message:      "back-off restarting container",
			},
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
				ErrorReason: ports.PodErrorReasonOOMKilled,
				ExitCode:    137,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, derivePodInfo(tt.pod))
		})
	}
}

func deployment(name string, replicas *int32, readyReplicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "project"},
		Spec:       appsv1.DeploymentSpec{Replicas: replicas},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: readyReplicas},
	}
}

func statefulSet(name string, replicas *int32, readyReplicas int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "project"},
		Spec:       appsv1.StatefulSetSpec{Replicas: replicas},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: readyReplicas},
	}
}

func int32Pointer(value int32) *int32 {
	return &value
}

func podNamed(name string) corev1.Pod {
	return corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func podWithStatuses(name string, statuses ...corev1.ContainerStatus) corev1.Pod {
	pod := podNamed(name)
	pod.Status.ContainerStatuses = statuses
	return pod
}
