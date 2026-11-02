package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Kinds of the objects the adapters compare against an object reference
// (e.g. corev1.Event.InvolvedObject.Kind). client-go exports no constant for
// them.
const (
	kindPod         = "Pod"
	kindDeployment  = "Deployment"
	kindReplicaSet  = "ReplicaSet"
	kindStatefulSet = "StatefulSet"
)

// currentObject returns the object ref designates, or nil when it no longer
// exists: deleted, or replaced by another object of the same name (a
// StatefulSet's pod keeps its name across restarts), which the UID tells
// apart. A ref without UID is matched by name only. Unknown kinds yield nil.
func currentObject(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	ref corev1.ObjectReference,
) (metav1.Object, error) {
	var obj metav1.Object
	var err error
	opts := metav1.GetOptions{}
	switch ref.Kind {
	case kindPod:
		obj, err = client.CoreV1().Pods(namespace).Get(ctx, ref.Name, opts)
	case kindReplicaSet:
		obj, err = client.AppsV1().ReplicaSets(namespace).Get(ctx, ref.Name, opts)
	case kindStatefulSet:
		obj, err = client.AppsV1().StatefulSets(namespace).Get(ctx, ref.Name, opts)
	case kindDeployment:
		obj, err = client.AppsV1().Deployments(namespace).Get(ctx, ref.Name, opts)
	default:
		return nil, nil
	}
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get %s %s: %w", ref.Kind, ref.Name, err)
	}
	if ref.UID != "" && obj.GetUID() != ref.UID {
		return nil, nil
	}
	return obj, nil
}
