package demo

import (
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func webPodTemplateLabels() map[string]string {
	return map[string]string{"app": "web", "pod-template-hash": "7d8f9c6b5"}
}

func webContainerSpec() corev1.Container {
	return corev1.Container{
		Name:  "web",
		Image: "ghcr.io/example/web:1.4.2",
		Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
		},
	}
}

func buildDeployment() *appsv1.Deployment {
	created := demoEpoch.Add(-7 * 24 * time.Hour)
	labels := map[string]string{"app": "web"}
	replicas := int32(3)
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              DeploymentWeb,
			Namespace:         NamespaceDemo,
			UID:               k8stypes.UID(uidDeploymentWeb),
			Labels:            labels,
			CreationTimestamp: metav1.NewTime(created),
			ManagedFields: []metav1.ManagedFieldsEntry{
				managedField("kubectl-client-side-apply", "Update",
					`{"f:spec":{"f:replicas":{},"f:selector":{},"f:template":{}}}`, created),
				managedField("kube-controller-manager", "Update",
					`{"f:status":{"f:readyReplicas":{},"f:replicas":{},"f:availableReplicas":{},"f:conditions":{}}}`,
					demoEpoch.Add(-5*time.Minute)),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: webPodTemplateLabels()},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{webContainerSpec()}},
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          3,
			ReadyReplicas:     2,
			AvailableReplicas: 2,
			UpdatedReplicas:   3,
			Conditions: []appsv1.DeploymentCondition{
				{
					Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue,
					Reason: "MinimumReplicasAvailable", Message: "Deployment has minimum availability.",
				},
				{
					Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue,
					Reason:  "NewReplicaSetAvailable",
					Message: `ReplicaSet "` + ReplicaSetWeb + `" has successfully progressed.`,
				},
			},
		},
	}
}

func buildReplicaSet() *appsv1.ReplicaSet {
	created := demoEpoch.Add(-7 * 24 * time.Hour)
	labels := webPodTemplateLabels()
	replicas := int32(3)
	controller, block := true, true
	return &appsv1.ReplicaSet{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "ReplicaSet"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              ReplicaSetWeb,
			Namespace:         NamespaceDemo,
			UID:               k8stypes.UID(uidReplicaSetWeb),
			Labels:            labels,
			CreationTimestamp: metav1.NewTime(created),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "apps/v1", Kind: "Deployment", Name: DeploymentWeb,
					UID: k8stypes.UID(uidDeploymentWeb), Controller: &controller, BlockOwnerDeletion: &block,
				},
			},
			ManagedFields: []metav1.ManagedFieldsEntry{
				managedField("deployment-controller", "Update",
					`{"f:metadata":{"f:ownerReferences":{}},"f:spec":{"f:replicas":{},"f:selector":{}}}`, created),
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{webContainerSpec()}},
			},
		},
		Status: appsv1.ReplicaSetStatus{Replicas: 3, ReadyReplicas: 2, AvailableReplicas: 2},
	}
}

func buildWebPods() []*corev1.Pod {
	return []*corev1.Pod{
		healthyWebPod(PodWebHealthy1, uidPodWebHealthy1, "10.0.2.11"),
		healthyWebPod(PodWebHealthy2, uidPodWebHealthy2, "10.0.2.12"),
		crashLoopWebPod(),
	}
}

func webPodOwnerRef() metav1.OwnerReference {
	controller, block := true, true
	return metav1.OwnerReference{
		APIVersion: "apps/v1", Kind: "ReplicaSet", Name: ReplicaSetWeb,
		UID: k8stypes.UID(uidReplicaSetWeb), Controller: &controller, BlockOwnerDeletion: &block,
	}
}

func healthyWebPod(name, uid, ip string) *corev1.Pod {
	started := demoEpoch.Add(-2 * time.Hour)
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         NamespaceDemo,
			UID:               k8stypes.UID(uid),
			Labels:            webPodTemplateLabels(),
			CreationTimestamp: metav1.NewTime(started),
			OwnerReferences:   []metav1.OwnerReference{webPodOwnerRef()},
			ManagedFields: []metav1.ManagedFieldsEntry{
				managedField("kube-controller-manager", "Update",
					`{"f:metadata":{"f:ownerReferences":{}},"f:spec":{"f:containers":{}}}`, started),
				managedField("kubelet", "Update",
					`{"f:status":{"f:containerStatuses":{},"f:podIP":{},"f:phase":{}}}`, demoEpoch.Add(-time.Minute)),
			},
		},
		Spec: corev1.PodSpec{
			NodeName:   NodeWorker1,
			Containers: []corev1.Container{webContainerSpec()},
		},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			PodIP:      ip,
			StartTime:  ptrTime(started),
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "web",
					Ready:        true,
					RestartCount: 0,
					Image:        "ghcr.io/example/web:1.4.2",
					State:        corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(started)}},
				},
			},
		},
	}
}

func crashLoopWebPod() *corev1.Pod {
	started := demoEpoch.Add(-40 * time.Minute)
	lastRestart := demoEpoch.Add(-3 * time.Minute)
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              PodWebCrashLoop,
			Namespace:         NamespaceDemo,
			UID:               k8stypes.UID(uidPodWebCrash),
			Labels:            webPodTemplateLabels(),
			CreationTimestamp: metav1.NewTime(started),
			OwnerReferences:   []metav1.OwnerReference{webPodOwnerRef()},
			ManagedFields: []metav1.ManagedFieldsEntry{
				managedField("kube-controller-manager", "Update",
					`{"f:metadata":{"f:ownerReferences":{}},"f:spec":{"f:containers":{}}}`, started),
				managedField("kubelet", "Update",
					`{"f:status":{"f:containerStatuses":{},"f:podIP":{},"f:phase":{}}}`, lastRestart),
			},
		},
		Spec: corev1.PodSpec{
			NodeName:   NodeWorker1,
			Containers: []corev1.Container{webContainerSpec()},
		},
		Status: corev1.PodStatus{
			Phase:     corev1.PodRunning,
			PodIP:     "10.0.2.13",
			StartTime: ptrTime(started),
			Conditions: []corev1.PodCondition{
				{
					Type: corev1.PodReady, Status: corev1.ConditionFalse,
					Reason: "ContainersNotReady", Message: "containers with unready status: [web]",
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "web",
					Ready:        false,
					RestartCount: 7,
					Image:        "ghcr.io/example/web:1.4.2",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "CrashLoopBackOff",
							Message: "back-off 2m40s restarting failed container=web pod=" +
								PodWebCrashLoop + "_" + NamespaceDemo + "(" + uidPodWebCrash + ")",
						},
					},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode:   1,
							Reason:     "Error",
							StartedAt:  metav1.NewTime(lastRestart.Add(-5 * time.Second)),
							FinishedAt: metav1.NewTime(lastRestart),
						},
					},
				},
			},
		},
	}
}
