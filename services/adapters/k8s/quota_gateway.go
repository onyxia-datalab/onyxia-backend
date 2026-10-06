package k8s

import (
	"context"
	"fmt"
	"sort"

	"github.com/onyxia-datalab/onyxia-backend/services/domain"
	"github.com/onyxia-datalab/onyxia-backend/services/ports"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type K8sQuotaGateway struct {
	client kubernetes.Interface
}

var _ ports.QuotaGateway = (*K8sQuotaGateway)(nil)

func NewQuotaGtw(client kubernetes.Interface) *K8sQuotaGateway {
	return &K8sQuotaGateway{client: client}
}

// GetProjectQuota merges the namespace's ResourceQuotas: for a resource
// several of them limit, the one with the smallest limit is reported.
func (g *K8sQuotaGateway) GetProjectQuota(ctx context.Context, namespace string) (domain.ProjectQuota, error) {
	quotas, err := g.client.CoreV1().ResourceQuotas(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return domain.ProjectQuota{}, fmt.Errorf("list resource quotas: %w", err)
	}
	return mergeQuotas(quotas.Items), nil
}

func mergeQuotas(quotas []corev1.ResourceQuota) domain.ProjectQuota {
	type limit struct{ hard, used resource.Quantity }
	limits := map[corev1.ResourceName]limit{}
	for _, q := range quotas {
		// Status.Hard is what the quota controller enforces; it lags behind
		// Spec.Hard only until the controller has seen the quota.
		hard := q.Status.Hard
		if len(hard) == 0 {
			hard = q.Spec.Hard
		}
		for name, h := range hard {
			if prev, ok := limits[name]; ok && prev.hard.Cmp(h) <= 0 {
				continue
			}
			limits[name] = limit{hard: h, used: q.Status.Used[name]}
		}
	}

	resources := make([]domain.QuotaResource, 0, len(limits))
	for name, l := range limits {
		resources = append(resources, domain.QuotaResource{
			Name: string(name),
			Hard: l.hard.String(),
			Used: l.used.String(),
		})
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].Name < resources[j].Name })
	return domain.ProjectQuota{Resources: resources}
}
