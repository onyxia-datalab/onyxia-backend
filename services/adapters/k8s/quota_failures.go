package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	reasonFailedCreate     = "FailedCreate"
	reasonSuccessfulCreate = "SuccessfulCreate"
)

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

	failures := map[string]string{}
	for ref, message := range quotaBlockedWorkloads(events.Items) {
		// The events of a deleted workload outlive it: only the current
		// object counts, not an earlier one of the same name.
		obj, err := currentObject(ctx, g.client, namespace, ref)
		if err != nil {
			return nil, err
		}
		if obj == nil {
			continue
		}
		if release := obj.GetLabels()[labelHelmInstance]; release != "" {
			failures[release] = message
		}
	}
	return failures, nil
}

// quotaBlockedWorkloads returns, for each workload controller whose last pod
// creation was refused by a quota with no successful creation since, the
// message of that refusal.
func quotaBlockedWorkloads(events []corev1.Event) map[corev1.ObjectReference]string {
	lastFailure := map[corev1.ObjectReference]corev1.Event{}
	lastSuccess := map[corev1.ObjectReference]time.Time{}
	for _, e := range events {
		ref := corev1.ObjectReference{Kind: e.InvolvedObject.Kind, Name: e.InvolvedObject.Name, UID: e.InvolvedObject.UID}
		if ref.Kind != kindReplicaSet && ref.Kind != kindStatefulSet {
			continue
		}
		switch {
		case e.Reason == reasonFailedCreate && isQuotaExceeded(e.Message):
			if prev, ok := lastFailure[ref]; !ok || eventTime(e).After(eventTime(prev)) {
				lastFailure[ref] = e
			}
		case e.Reason == reasonSuccessfulCreate && eventTime(e).After(lastSuccess[ref]):
			lastSuccess[ref] = eventTime(e)
		}
	}

	blocked := map[corev1.ObjectReference]string{}
	for ref, e := range lastFailure {
		if eventTime(e).After(lastSuccess[ref]) {
			blocked[ref] = e.Message
		}
	}
	return blocked
}

// isQuotaExceeded reports whether a pod creation failure is due to a
// ResourceQuota, whose admission error reads "... exceeded quota: ...".
func isQuotaExceeded(message string) bool {
	return strings.Contains(message, "exceeded quota")
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
