package demo

import (
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func buildJob() *batchv1.Job {
	started := demoEpoch.Add(-25 * time.Minute)
	backoffLimit := int32(4)
	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              JobDBMigrate,
			Namespace:         NamespaceJobs,
			UID:               k8stypes.UID(uidJobDBMigrate),
			Labels:            map[string]string{"job-name": JobDBMigrate},
			CreationTimestamp: metav1.NewTime(started),
			ManagedFields: []metav1.ManagedFieldsEntry{
				managedField("kubectl-client-side-apply", "Create",
					`{"f:spec":{"f:backoffLimit":{},"f:template":{}}}`, started),
				managedField("kube-controller-manager", "Update",
					`{"f:status":{"f:conditions":{},"f:failed":{},"f:startTime":{}}}`, demoEpoch.Add(-2*time.Minute)),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"job-name": JobDBMigrate}},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{Name: "migrate", Image: "ghcr.io/example/db-migrate:2.3.0"},
					},
				},
			},
		},
		Status: batchv1.JobStatus{
			Failed:    3,
			StartTime: ptrTime(started),
			Conditions: []batchv1.JobCondition{
				{
					Type: batchv1.JobFailed, Status: corev1.ConditionTrue,
					Reason: "BackoffLimitExceeded", Message: "Job has reached the specified backoff limit",
				},
			},
		},
	}
}

func buildJobPod() *corev1.Pod {
	started := demoEpoch.Add(-25 * time.Minute)
	finished := demoEpoch.Add(-22 * time.Minute)
	controller, block := true, true
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              PodDBMigrate,
			Namespace:         NamespaceJobs,
			UID:               k8stypes.UID(uidPodDBMigrate),
			Labels:            map[string]string{"job-name": JobDBMigrate},
			CreationTimestamp: metav1.NewTime(started),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "batch/v1", Kind: "Job", Name: JobDBMigrate,
					UID: k8stypes.UID(uidJobDBMigrate), Controller: &controller, BlockOwnerDeletion: &block,
				},
			},
			ManagedFields: []metav1.ManagedFieldsEntry{
				managedField("kube-controller-manager", "Update",
					`{"f:metadata":{"f:ownerReferences":{}}}`, started),
				managedField("kubelet", "Update",
					`{"f:status":{"f:containerStatuses":{},"f:phase":{}}}`, finished),
			},
		},
		Spec: corev1.PodSpec{
			NodeName:      NodeWorker1,
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{Name: "migrate", Image: "ghcr.io/example/db-migrate:2.3.0"},
			},
		},
		Status: corev1.PodStatus{
			Phase:     corev1.PodFailed,
			StartTime: ptrTime(started),
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "migrate",
					Ready:        false,
					RestartCount: 0,
					Image:        "ghcr.io/example/db-migrate:2.3.0",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode:   1,
							Reason:     "Error",
							Message:    `migration failed: relation "accounts" already exists`,
							StartedAt:  metav1.NewTime(started),
							FinishedAt: metav1.NewTime(finished),
						},
					},
				},
			},
		},
	}
}
