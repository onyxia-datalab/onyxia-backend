package k8s

// Kinds of the objects the adapters compare against an object reference
// (e.g. corev1.Event.InvolvedObject.Kind). client-go exports no constant for
// them.
const (
	kindPod         = "Pod"
	kindDeployment  = "Deployment"
	kindReplicaSet  = "ReplicaSet"
	kindStatefulSet = "StatefulSet"
)
