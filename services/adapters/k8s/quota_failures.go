package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	kindReplicaSet  = "ReplicaSet"
	kindStatefulSet = "StatefulSet"

	reasonFailedCreate     = "FailedCreate"
	reasonSuccessfulCreate = "SuccessfulCreate"
)

// workloadRef identifies a workload controller that creates pods.
type workloadRef struct {
	kind string
	name string
}

// ListQuotaFailures reads the namespace's pod creation events. A workload
// controller (ReplicaSet, StatefulSet) is blocked by the quota when its last
// FailedCreate mentioning an exceeded quota is more recent than its last
// SuccessfulCreate: the controller keeps retrying, and the failure is over
// as soon as a creation succeeds. Its release is read from its Helm instance
// label. Events expire (one hour by default): an older failure is missed.
func (g *K8sWorkloadStateGateway) ListQuotaFailures(
	ctx context.Context,
	namespace string,
) (map[string]string, error) {
	events, err := g.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	lastFailure := map[workloadRef]corev1.Event{}
	lastSuccess := map[workloadRef]time.Time{}
	for _, e := range events.Items {
		ref := workloadRef{kind: e.InvolvedObject.Kind, name: e.InvolvedObject.Name}
		if ref.kind != kindReplicaSet && ref.kind != kindStatefulSet {
			continue
		}
		switch {
		case e.Reason == reasonFailedCreate && isQuotaExceeded(e.Message):
			if prev, ok := lastFailure[ref]; !ok || eventTime(e).After(eventTime(prev)) {
				lastFailure[ref] = e
			}
		case e.Reason == reasonSuccessfulCreate:
			if t := eventTime(e); t.After(lastSuccess[ref]) {
				lastSuccess[ref] = t
			}
		}
	}

	failures := map[string]string{}
	for ref, e := range lastFailure {
		if !eventTime(e).After(lastSuccess[ref]) {
			continue
		}
		release, err := g.workloadRelease(ctx, namespace, ref)
		if err != nil {
			return nil, err
		}
		if release != "" {
			failures[release] = e.Message
		}
	}
	return failures, nil
}

// isQuotaExceeded reports whether a pod creation failure is due to a
// ResourceQuota, whose admission error reads "... exceeded quota: ...".
func isQuotaExceeded(message string) bool {
	return strings.Contains(message, "exceeded quota")
}

// workloadRelease returns the release a workload controller belongs to, or
// "" when it no longer exists or isn't a Helm release's.
func (g *K8sWorkloadStateGateway) workloadRelease(ctx context.Context, namespace string, ref workloadRef) (string, error) {
	var meta metav1.Object
	var err error
	switch ref.kind {
	case kindReplicaSet:
		meta, err = g.client.AppsV1().ReplicaSets(namespace).Get(ctx, ref.name, metav1.GetOptions{})
	case kindStatefulSet:
		meta, err = g.client.AppsV1().StatefulSets(namespace).Get(ctx, ref.name, metav1.GetOptions{})
	default:
		return "", nil
	}
	if apierrors.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get %s %s: %w", ref.kind, ref.name, err)
	}
	return meta.GetLabels()[labelHelmInstance], nil
}

// eventTime is when an event last occurred.
func eventTime(e corev1.Event) time.Time {
	switch {
	case !e.LastTimestamp.IsZero():
		return e.LastTimestamp.Time
	case !e.EventTime.IsZero():
		return e.EventTime.Time
	default:
		return e.CreationTimestamp.Time
	}
}
