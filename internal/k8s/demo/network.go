package demo

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func buildService() *corev1.Service {
	created := demoEpoch.Add(-7 * 24 * time.Hour)
	return &corev1.Service{
		APIVersion: "v1", Kind: "Service",
		Name:              ServiceWeb,
		Namespace:         NamespaceDemo,
		UID:               k8stypes.UID(uidServiceWeb),
		Labels:            map[string]string{"app": "web"},
		CreationTimestamp: metav1.NewTime(created),
		ManagedFields: []metav1.ManagedFieldsEntry{
			managedField("kubectl-client-side-apply", "Update",
				`{"f:spec":{"f:ports":{},"f:selector":{},"f:type":{}}}`, created),
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "10.96.0.42",
			Selector:  map[string]string{"app": "web"},
			Ports: []corev1.ServicePort{
				{Name: "http", Port: 80, TargetPort: intstr.FromInt(8080), Protocol: corev1.ProtocolTCP},
			},
		},
	}
}

func buildConfigMap() *corev1.ConfigMap {
	created := demoEpoch.Add(-7 * 24 * time.Hour)
	return &corev1.ConfigMap{
		APIVersion: "v1", Kind: "ConfigMap",
		Name:              ConfigMapWeb,
		Namespace:         NamespaceDemo,
		UID:               k8stypes.UID(uidConfigMapWeb),
		CreationTimestamp: metav1.NewTime(created),
		ManagedFields: []metav1.ManagedFieldsEntry{
			managedField("kubectl-client-side-apply", "Update", `{"f:data":{}}`, created),
		},
		Data: map[string]string{
			"APP_ENV":   "production",
			"LOG_LEVEL": "info",
		},
	}
}
