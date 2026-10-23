package kubernetes

import (
	"context"
	"fmt"

	"github.com/onyxia-datalab/onyxia-backend/onboarding/domain"
	"github.com/onyxia-datalab/onyxia-backend/onboarding/ports"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

// FieldManager owns, in server-side apply terms, the fields set by the
// configured resources.
const FieldManager string = "onyxia-onboarding"

type KubernetesResourceApplier struct {
	client dynamic.Interface
	mapper meta.RESTMapper
}

// NewKubernetesResourceApplier takes a mapper backed by the cluster's
// discovery, so that kinds of CRDs are resolved too.
func NewKubernetesResourceApplier(client dynamic.Interface, mapper meta.RESTMapper) ports.ResourceApplier {
	return &KubernetesResourceApplier{client: client, mapper: mapper}
}

func (a *KubernetesResourceApplier) CheckResource(resource domain.Resource) error {
	_, err := a.namespacedMapping(&unstructured.Unstructured{Object: resource})
	return err
}

// ApplyResource uses a forced server-side apply: fields set by the resource
// are taken over from whoever changed them, fields it does not set are left
// alone.
func (a *KubernetesResourceApplier) ApplyResource(
	ctx context.Context,
	namespace string,
	resource domain.Resource,
) error {
	obj := &unstructured.Unstructured{Object: runtime.DeepCopyJSON(resource)}
	obj.SetNamespace(namespace)

	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	if _, ok := labels["created-by"]; !ok {
		labels["created-by"] = "onyxia"
	}
	obj.SetLabels(labels)

	mapping, err := a.namespacedMapping(obj)
	if err != nil {
		return err
	}

	_, err = a.client.Resource(mapping.Resource).Namespace(namespace).Apply(
		ctx,
		obj.GetName(),
		obj,
		metav1.ApplyOptions{FieldManager: FieldManager, Force: true},
	)
	if err != nil {
		return fmt.Errorf("failed to apply %s %q: %w", obj.GetKind(), obj.GetName(), err)
	}
	return nil
}

func (a *KubernetesResourceApplier) namespacedMapping(obj *unstructured.Unstructured) (*meta.RESTMapping, error) {
	gvk := obj.GroupVersionKind()
	mapping, err := a.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("unknown kind %s: %w", gvk, err)
	}
	if mapping.Scope.Name() != meta.RESTScopeNameNamespace {
		return nil, fmt.Errorf("kind %s is cluster-scoped: only namespaced kinds can be applied", gvk)
	}
	return mapping, nil
}
