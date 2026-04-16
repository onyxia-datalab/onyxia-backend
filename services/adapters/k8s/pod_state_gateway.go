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
	for _, r := range resources {
		switch r.Kind {
		case "Deployment":
			d, err := g.client.AppsV1().Deployments(namespace).Get(ctx, r.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			desired := int32(1)
			if d.Spec.Replicas != nil {
				desired = *d.Spec.Replicas
			}
			if d.Status.ReadyReplicas < desired {
				return false, nil
			}
		case "StatefulSet":
			s, err := g.client.AppsV1().StatefulSets(namespace).Get(ctx, r.Name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			desired := int32(1)
			if s.Spec.Replicas != nil {
				desired = *s.Spec.Replicas
			}
			if s.Status.ReadyReplicas < desired {
				return false, nil
			}
		}
	}
	return true, nil
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

	// Check if pod is unschedulable.
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse &&
			cond.Reason == "Unschedulable" {
			info.ErrorReason = ports.PodErrorReasonUnschedulable
			info.Message = cond.Message
			return info
		}
	}

	// Inspect container statuses for errors.
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			switch reason {
			case "CrashLoopBackOff":
				if info.ErrorReason == "" || errorPriority(ports.PodErrorReasonCrashLoop) > errorPriority(info.ErrorReason) {
					info.ErrorReason = ports.PodErrorReasonCrashLoop
					info.RestartCount = cs.RestartCount
					info.Message = cs.State.Waiting.Message
				}
			case "ImagePullBackOff", "ErrImagePull":
				if info.ErrorReason == "" || errorPriority(ports.PodErrorReasonImagePull) > errorPriority(info.ErrorReason) {
					info.ErrorReason = ports.PodErrorReasonImagePull
					info.Image = cs.Image
					info.Message = cs.State.Waiting.Message
				}
			case "CreateContainerConfigError":
				if info.ErrorReason == "" || errorPriority(ports.PodErrorReasonConfigError) > errorPriority(info.ErrorReason) {
					info.ErrorReason = ports.PodErrorReasonConfigError
					info.Message = cs.State.Waiting.Message
				}
			}
		}

		if cs.State.Terminated != nil && cs.State.Terminated.Reason == "OOMKilled" {
			if info.ErrorReason == "" || errorPriority(ports.PodErrorReasonOOMKilled) > errorPriority(info.ErrorReason) {
				info.ErrorReason = ports.PodErrorReasonOOMKilled
				info.ExitCode = cs.State.Terminated.ExitCode
			}
		}
	}

	if info.ErrorReason != "" {
		return info
	}

	// Check readiness: pod running but not all containers ready.
	allReady := true
	for _, cs := range pod.Status.ContainerStatuses {
		if !cs.Ready {
			allReady = false
			if cs.State.Running != nil {
				info.ErrorReason = ports.PodErrorReasonReadinessFailed
				info.Name = pod.Name
			}
		}
	}
	info.Ready = allReady

	return info
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
