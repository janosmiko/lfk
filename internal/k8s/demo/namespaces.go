package demo

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// buildNamespaces returns a Namespace object for every namespace the seed
// data uses (NamespaceDemo, NamespaceJobs), plus the two namespaces every
// real cluster has (default, kube-system). This way the namespace picker
// and GetNamespaces count reflect a coherent cluster instead of reporting
// zero.
func buildNamespaces() []*corev1.Namespace {
	return []*corev1.Namespace{
		namespace(NamespaceDemo, uidNamespaceDemo, demoEpoch.Add(-30*24*time.Hour)),
		namespace(NamespaceJobs, uidNamespaceJobs, demoEpoch.Add(-30*24*time.Hour)),
		namespace(NamespaceDefault, uidNamespaceDefault, demoEpoch.Add(-60*24*time.Hour)),
		namespace(NamespaceKubeSystem, uidNamespaceKubeSystem, demoEpoch.Add(-60*24*time.Hour)),
	}
}

func namespace(name, uid string, created time.Time) *corev1.Namespace {
	return &corev1.Namespace{
		APIVersion: "v1", Kind: "Namespace",
		Name:              name,
		UID:               k8stypes.UID(uid),
		Labels:            map[string]string{"kubernetes.io/metadata.name": name},
		CreationTimestamp: metav1.NewTime(created),
		ManagedFields: []metav1.ManagedFieldsEntry{
			managedField("kube-apiserver", "Update", `{"f:status":{"f:phase":{}}}`, created),
		},
		Status: corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
	}
}
