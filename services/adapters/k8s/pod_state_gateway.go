package k8s

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetControllerReadiness returns true when all Deployments and StatefulSets in the
// provided resource list have their desired replica count ready.
// Other resource kinds are ignored.
func (g *K8sWorkloadStateGateway) GetControllerReadiness(
	ctx context.Context,
	namespace string,
	resources []ports.ManifestResource,
) (bool, error) {
	for _, resource := range resources {
		ready, handled, err := g.controllerReady(ctx, namespace, resource)
		if err != nil {
			return false, err
		}
		if handled && !ready {
			return false, nil
		}
	}
	return true, nil
}

func (g *K8sWorkloadStateGateway) controllerReady(
	ctx context.Context,
	namespace string,
	resource ports.ManifestResource,
) (ready bool, handled bool, err error) {
	switch resource.Kind {
	case "Deployment":
		deployment, err := g.client.AppsV1().Deployments(namespace).
			Get(ctx, resource.Name, metav1.GetOptions{})
		if err != nil {
			return false, true, err
		}
		return replicasReady(deployment.Spec.Replicas, deployment.Status.ReadyReplicas), true, nil
	case "StatefulSet":
		statefulSet, err := g.client.AppsV1().StatefulSets(namespace).
			Get(ctx, resource.Name, metav1.GetOptions{})
		if err != nil {
			return false, true, err
		}
		return replicasReady(statefulSet.Spec.Replicas, statefulSet.Status.ReadyReplicas), true, nil
	default:
		return false, false, nil
	}
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

	infos := make([]ports.PodInfo, 0, len(list.Items))
	for _, pod := range list.Items {
		infos = append(infos, derivePodInfo(pod))
	}
	return infos, nil
}

// derivePodInfo inspects a pod's conditions and container statuses to produce a PodInfo.
// Error priority (highest first): CrashLoopBackOff > OOMKilled > ImagePull > ConfigError > Unschedulable > ReadinessFailed.
func derivePodInfo(pod corev1.Pod) ports.PodInfo {
	info := ports.PodInfo{Name: pod.Name}

	updatePodError(&info, unschedulableError(pod.Status.Conditions))
	for _, status := range pod.Status.ContainerStatuses {
		updatePodError(&info, waitingContainerError(status))
		updatePodError(&info, terminatedContainerError(status.State.Terminated))
		updatePodError(&info, terminatedContainerError(status.LastTerminationState.Terminated))
	}

	if info.ErrorReason != "" {
		return info
	}

	applyContainerReadiness(&info, pod.Status.ContainerStatuses)

	return info
}

func unschedulableError(conditions []corev1.PodCondition) ports.PodInfo {
	for _, condition := range conditions {
		if condition.Type == corev1.PodScheduled &&
			condition.Status == corev1.ConditionFalse &&
			condition.Reason == "Unschedulable" {
			return ports.PodInfo{
				ErrorReason: ports.PodErrorReasonUnschedulable,
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
			ErrorReason:  ports.PodErrorReasonCrashLoop,
			RestartCount: status.RestartCount,
			Message:      waiting.Message,
		}
	case "ImagePullBackOff", "ErrImagePull":
		return ports.PodInfo{
			ErrorReason: ports.PodErrorReasonImagePull,
			Image:       status.Image,
			Message:     waiting.Message,
		}
	case "CreateContainerConfigError":
		return ports.PodInfo{
			ErrorReason: ports.PodErrorReasonConfigError,
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
		ErrorReason: ports.PodErrorReasonOOMKilled,
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

func applyContainerReadiness(info *ports.PodInfo, statuses []corev1.ContainerStatus) {
	if len(statuses) == 0 {
		return
	}

	info.Ready = true
	for _, status := range statuses {
		if status.Ready {
			continue
		}
		info.Ready = false
		if status.State.Running != nil {
			info.ErrorReason = ports.PodErrorReasonReadinessFailed
		}
	}
}

// errorPriority returns the severity of a pod error reason (higher = more severe).
func errorPriority(r ports.PodErrorReason) int {
	switch r {
	case ports.PodErrorReasonCrashLoop:
		return 5
	case ports.PodErrorReasonOOMKilled:
		return 4
	case ports.PodErrorReasonImagePull:
		return 3
	case ports.PodErrorReasonConfigError:
		return 2
	case ports.PodErrorReasonUnschedulable:
		return 1
	default:
		return 0
	}
}
