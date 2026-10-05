package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetWorkloadReadiness lists the Deployments and StatefulSets of the
// namespace once; the returned snapshot answers for any number of releases.
func (g *K8sWorkloadStateGateway) GetWorkloadReadiness(
	ctx context.Context,
	namespace string,
) (ports.WorkloadReadiness, error) {
	snapshot := workloadSnapshot{}

	deployments, err := g.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	for _, d := range deployments.Items {
		snapshot[ports.ManifestResource{Kind: kindDeployment, Name: d.Name}] =
			replicasReady(d.Spec.Replicas, d.Status.ReadyReplicas)
	}

	statefulSets, err := g.client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list statefulsets: %w", err)
	}
	for _, s := range statefulSets.Items {
		snapshot[ports.ManifestResource{Kind: kindStatefulSet, Name: s.Name}] =
			replicasReady(s.Spec.Replicas, s.Status.ReadyReplicas)
	}

	return snapshot, nil
}

const (
	kindDeployment  = "Deployment"
	kindStatefulSet = "StatefulSet"
)

// workloadSnapshot maps each workload controller of a namespace to its
// readiness.
type workloadSnapshot map[ports.ManifestResource]bool

// AllReady implements ports.WorkloadReadiness: only Deployments and
// StatefulSets are considered, and one missing from the namespace (not
// created yet, or deleted) is not ready.
func (s workloadSnapshot) AllReady(resources []ports.ManifestResource) bool {
	for _, r := range resources {
		if r.Kind != kindDeployment && r.Kind != kindStatefulSet {
			continue
		}
		if !s[r] {
			return false
		}
	}
	return true
}

func replicasReady(replicas *int32, readyReplicas int32) bool {
	desiredReplicas := int32(1)
	if replicas != nil {
		desiredReplicas = *replicas
	}
	return readyReplicas >= desiredReplicas
}

var _ ports.WorkloadStateGateway = (*K8sWorkloadStateGateway)(nil)

// labelHelmInstance is the standard Helm label used to associate pods with a release.
// Onyxia charts emit it via the library-chart helper:
// https://github.com/InseeFrLab/helm-charts-interactive-services/blob/daebffd19af39e8fcd1f21fb0ec9fc902b77301e/charts/library-chart/templates/_label.tpl#L20
const labelHelmInstance = "app.kubernetes.io/instance"

type K8sWorkloadStateGateway struct {
	client kubernetes.Interface
}

func NewWorkloadStateGtw(client kubernetes.Interface) *K8sWorkloadStateGateway {
	return &K8sWorkloadStateGateway{client: client}
}

func (g *K8sWorkloadStateGateway) GetPodsForRelease(
	ctx context.Context,
	namespace, releaseID string,
) ([]ports.PodInfo, error) {
	list, err := g.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", labelHelmInstance, releaseID),
	})
	if err != nil {
		return nil, err
	}

	if len(list.Items) == 0 {
		slog.WarnContext(ctx, "no pods found for release — chart may be missing the standard Helm labels",
			slog.String("release", releaseID),
			slog.String("namespace", namespace),
			slog.String("label", labelHelmInstance),
		)
	}

	now := time.Now()
	infos := make([]ports.PodInfo, 0, len(list.Items))
	for _, pod := range list.Items {
		infos = append(infos, derivePodInfo(pod, now))
	}
	return infos, nil
}

// readinessGracePeriod is how long a running container may stay not ready
// before it is reported as ReadinessFailed. Until then it is considered to
// be starting (readiness probes of interactive services can take a while).
const readinessGracePeriod = 5 * time.Minute

// derivePodInfo inspects a pod's conditions and container statuses to produce a PodInfo.
// Error priority (highest first): CrashLoopBackOff > OOMKilled > ImagePull > ConfigError > Unschedulable > ReadinessFailed.
func derivePodInfo(pod corev1.Pod, now time.Time) ports.PodInfo {
	info := ports.PodInfo{Name: pod.Name}

	updatePodError(&info, unschedulableError(pod.Status.Conditions))
	for _, status := range pod.Status.ContainerStatuses {
		updatePodError(&info, waitingContainerError(status))
		updatePodError(&info, terminatedContainerError(status.State.Terminated))
		// Kubernetes keeps the last termination until the next one: once the
		// container is ready again, a past OOMKill is history, not an error.
		if !status.Ready {
			updatePodError(&info, terminatedContainerError(status.LastTerminationState.Terminated))
		}
	}

	if info.ErrorReason != "" {
		return info
	}

	applyContainerReadiness(&info, pod.Status.ContainerStatuses, now)

	return info
}

func unschedulableError(conditions []corev1.PodCondition) ports.PodInfo {
	for _, condition := range conditions {
		if condition.Type == corev1.PodScheduled &&
			condition.Status == corev1.ConditionFalse &&
			condition.Reason == "Unschedulable" {
			return ports.PodInfo{
				ErrorReason: domain.ServiceErrorReasonUnschedulable,
				Message:     condition.Message,
			}
		}
	}
	return ports.PodInfo{}
}

func waitingContainerError(status corev1.ContainerStatus) ports.PodInfo {
	if status.State.Waiting == nil {
		return ports.PodInfo{}
	}

	waiting := status.State.Waiting
	switch waiting.Reason {
	case "CrashLoopBackOff":
		return ports.PodInfo{
			ErrorReason:  domain.ServiceErrorReasonCrashLoop,
			RestartCount: status.RestartCount,
			Message:      waiting.Message,
		}
	case "ImagePullBackOff", "ErrImagePull":
		return ports.PodInfo{
			ErrorReason: domain.ServiceErrorReasonImagePull,
			Image:       status.Image,
			Message:     waiting.Message,
		}
	case "CreateContainerConfigError":
		return ports.PodInfo{
			ErrorReason: domain.ServiceErrorReasonConfigError,
			Message:     waiting.Message,
		}
	default:
		return ports.PodInfo{}
	}
}

func terminatedContainerError(terminated *corev1.ContainerStateTerminated) ports.PodInfo {
	if terminated == nil || terminated.Reason != "OOMKilled" {
		return ports.PodInfo{}
	}
	return ports.PodInfo{
		ErrorReason: domain.ServiceErrorReasonOOMKilled,
		ExitCode:    terminated.ExitCode,
	}
}

func updatePodError(info *ports.PodInfo, candidate ports.PodInfo) {
	if candidate.ErrorReason == "" ||
		errorPriority(candidate.ErrorReason) <= errorPriority(info.ErrorReason) {
		return
	}
	candidate.Name = info.Name
	*info = candidate
}

func applyContainerReadiness(info *ports.PodInfo, statuses []corev1.ContainerStatus, now time.Time) {
	if len(statuses) == 0 {
		return
	}

	info.Ready = true
	for _, status := range statuses {
		if status.Ready {
			continue
		}
		info.Ready = false
		if running := status.State.Running; running != nil &&
			now.Sub(running.StartedAt.Time) > readinessGracePeriod {
			info.ErrorReason = domain.ServiceErrorReasonReadinessFailed
		}
	}
}

// errorPriority returns the severity of a pod error reason (higher = more severe).
func errorPriority(r domain.ServiceErrorReason) int {
	switch r {
	case domain.ServiceErrorReasonCrashLoop:
		return 5
	case domain.ServiceErrorReasonOOMKilled:
		return 4
	case domain.ServiceErrorReasonImagePull:
		return 3
	case domain.ServiceErrorReasonConfigError:
		return 2
	case domain.ServiceErrorReasonUnschedulable:
		return 1
	default:
		return 0
	}
}
