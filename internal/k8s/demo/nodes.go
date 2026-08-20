package demo

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func buildNodes() []*corev1.Node {
	return []*corev1.Node{
		node(NodeControlPlane, uidNodeControlPlane, map[string]string{
			"kubernetes.io/hostname":                NodeControlPlane,
			"node-role.kubernetes.io/control-plane": "",
		}),
		node(NodeWorker1, uidNodeWorker1, map[string]string{
			"kubernetes.io/hostname": NodeWorker1,
		}),
	}
}

func node(name, uid string, labels map[string]string) *corev1.Node {
	created := demoEpoch.Add(-30 * 24 * time.Hour)
	heartbeat := demoEpoch.Add(-time.Minute)
	return &corev1.Node{
		APIVersion: "v1", Kind: "Node",
		Name:              name,
		UID:               k8stypes.UID(uid),
		Labels:            labels,
		CreationTimestamp: metav1.NewTime(created),
		ManagedFields: []metav1.ManagedFieldsEntry{
			managedField("kubectl-client-side-apply", "Update", `{"f:metadata":{"f:labels":{}}}`, created),
			managedField("kubelet", "Update", `{"f:status":{"f:conditions":{},"f:nodeInfo":{}}}`, heartbeat),
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion:          "v1.31.2",
				OSImage:                 "Ubuntu 22.04.4 LTS",
				ContainerRuntimeVersion: "containerd://1.7.20",
				Architecture:            "amd64",
				OperatingSystem:         "linux",
			},
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("3800m"),
				corev1.ResourceMemory: resource.MustParse("15Gi"),
				corev1.ResourcePods:   resource.MustParse("110"),
			},
			Conditions: []corev1.NodeCondition{
				{
					Type:               corev1.NodeReady,
					Status:             corev1.ConditionTrue,
					Reason:             "KubeletReady",
					Message:            "kubelet is posting ready status",
					LastHeartbeatTime:  metav1.NewTime(heartbeat),
					LastTransitionTime: metav1.NewTime(created),
				},
			},
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "10.0.1.10"},
				{Type: corev1.NodeHostName, Address: name},
			},
		},
	}
}
