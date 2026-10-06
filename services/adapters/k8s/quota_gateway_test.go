package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestGetProjectQuotaKeepsTheMostRestrictive(t *testing.T) {
	gateway := NewQuotaGtw(k8sfake.NewClientset(
		resourceQuota("onyxia-quota",
			corev1.ResourceList{"requests.memory": resource.MustParse("10Gi"), "pods": resource.MustParse("20")},
			corev1.ResourceList{"requests.memory": resource.MustParse("2560Mi"), "pods": resource.MustParse("3")},
		),
		resourceQuota("admin-quota",
			corev1.ResourceList{"requests.memory": resource.MustParse("8Gi")},
			corev1.ResourceList{"requests.memory": resource.MustParse("2560Mi")},
		),
	))

	quota, err := gateway.GetProjectQuota(context.Background(), "project")

	require.NoError(t, err)
	assert.Equal(t, domain.ProjectQuota{Resources: []domain.QuotaResource{
		{Name: "pods", Hard: "20", Used: "3"},
		{Name: "requests.memory", Hard: "8Gi", Used: "2560Mi"},
	}}, quota)
}

func TestGetProjectQuotaWithoutQuota(t *testing.T) {
	quota, err := NewQuotaGtw(k8sfake.NewClientset()).GetProjectQuota(context.Background(), "project")

	require.NoError(t, err)
	assert.Empty(t, quota.Resources)
}

// Before the quota controller has seen a quota, only its spec is filled.
func TestGetProjectQuotaFallsBackToTheSpec(t *testing.T) {
	q := resourceQuota("new", nil, nil)
	q.Spec.Hard = corev1.ResourceList{"limits.cpu": resource.MustParse("4")}

	quota, err := NewQuotaGtw(k8sfake.NewClientset(q)).GetProjectQuota(context.Background(), "project")

	require.NoError(t, err)
	assert.Equal(t, []domain.QuotaResource{{Name: "limits.cpu", Hard: "4", Used: "0"}}, quota.Resources)
}

func TestListQuotaFailures(t *testing.T) {
	t0 := time.Date(2026, 10, 6, 10, 0, 0, 0, time.UTC)
	quotaMsg := `create Pod jupyter-0 in StatefulSet jupyter failed error: pods "jupyter-0" is forbidden: exceeded quota: onyxia-quota`
	client := k8sfake.NewClientset(
		labelledStatefulSet("jupyter", "jupyter"),
		labelledStatefulSet("vscode", "vscode"),
		labelledStatefulSet("rstudio", "rstudio"),
		// Blocked: the last creation failed on the quota.
		workloadEvent("e1", kindStatefulSet, "jupyter", reasonSuccessfulCreate, "created", t0),
		workloadEvent("e2", kindStatefulSet, "jupyter", reasonFailedCreate, quotaMsg, t0.Add(time.Minute)),
		// Resolved: a creation succeeded after the failure.
		workloadEvent("e3", kindStatefulSet, "vscode", reasonFailedCreate, quotaMsg, t0),
		workloadEvent("e4", kindStatefulSet, "vscode", reasonSuccessfulCreate, "created", t0.Add(time.Minute)),
		// Not a quota failure.
		workloadEvent("e5", kindStatefulSet, "rstudio", reasonFailedCreate, "admission webhook denied", t0),
		// The workload no longer exists.
		workloadEvent("e6", kindStatefulSet, "deleted", reasonFailedCreate, quotaMsg, t0),
	)

	failures, err := NewWorkloadStateGtw(client).ListQuotaFailures(context.Background(), "project")

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"jupyter": quotaMsg}, failures)
}

func resourceQuota(name string, hard, used corev1.ResourceList) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "project"},
		Status:     corev1.ResourceQuotaStatus{Hard: hard, Used: used},
	}
}

func labelledStatefulSet(name, release string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Name:      name,
		Namespace: "project",
		Labels:    map[string]string{labelHelmInstance: release},
	}}
}

func workloadEvent(name, kind, object, reason, message string, at time.Time) *corev1.Event {
	return &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: name, Namespace: "project"},
		InvolvedObject: corev1.ObjectReference{Kind: kind, Name: object, Namespace: "project"},
		Reason:         reason,
		Message:        message,
		Type:           corev1.EventTypeWarning,
		LastTimestamp:  metav1.Time{Time: at},
	}
}
