package demo

import (
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

// nightlyBackupContainer is shared by the CronJob's own template and its
// one seeded run, so both describe the same workload.
func nightlyBackupContainer() corev1.Container {
	return corev1.Container{Name: "backup", Image: "ghcr.io/example/nightly-backup:1.0.0"}
}

func buildCronJob() *batchv1.CronJob {
	created := demoEpoch.Add(-30 * 24 * time.Hour)
	lastSchedule := demoEpoch.Add(-25 * time.Minute)
	suspend := false
	return &batchv1.CronJob{
		APIVersion: "batch/v1", Kind: "CronJob",
		Name:              CronJobNightlyBackup,
		Namespace:         NamespaceJobs,
		UID:               k8stypes.UID(UIDCronJobNightlyBackup),
		CreationTimestamp: metav1.NewTime(created),
		ManagedFields: []metav1.ManagedFieldsEntry{
			managedField("kubectl-client-side-apply", "Update",
				`{"f:spec":{"f:schedule":{},"f:jobTemplate":{}}}`, created),
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 2 * * *",
			Suspend:  &suspend,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers:    []corev1.Container{nightlyBackupContainer()},
						},
					},
				},
			},
		},
		Status: batchv1.CronJobStatus{
			LastScheduleTime:   ptrTime(lastSchedule),
			LastSuccessfulTime: ptrTime(lastSchedule.Add(3 * time.Minute)),
		},
	}
}

// buildCronJobOwnedJob is owned by CronJobNightlyBackup via a Controller
// ownerReference - the shape latestOwnedJob (internal/app/commands_logs_cronjob.go) matches on.
func buildCronJobOwnedJob() *batchv1.Job {
	started := demoEpoch.Add(-25 * time.Minute)
	completed := demoEpoch.Add(-22 * time.Minute)
	controller, block := true, true
	return &batchv1.Job{
		APIVersion: "batch/v1", Kind: "Job",
		Name:              JobNightlyBackupRun,
		Namespace:         NamespaceJobs,
		UID:               k8stypes.UID(UIDJobNightlyBackupRun),
		CreationTimestamp: metav1.NewTime(started),
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "batch/v1", Kind: "CronJob", Name: CronJobNightlyBackup,
				UID: k8stypes.UID(UIDCronJobNightlyBackup), Controller: &controller, BlockOwnerDeletion: &block,
			},
		},
		ManagedFields: []metav1.ManagedFieldsEntry{
			managedField("cronjob-controller", "Update",
				`{"f:metadata":{"f:ownerReferences":{}},"f:spec":{"f:template":{}}}`, started),
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"batch.kubernetes.io/job-name": JobNightlyBackupRun}},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers:    []corev1.Container{nightlyBackupContainer()},
				},
			},
		},
		Status: batchv1.JobStatus{
			Succeeded:      1,
			StartTime:      ptrTime(started),
			CompletionTime: ptrTime(completed),
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}
}

// buildCronJobOwnedPod carries only the modern batch.kubernetes.io/-prefixed
// labels, the pair internal/democli/get_cronjob.go answers - the legacy
// fallback needs a real apiserver to exercise.
func buildCronJobOwnedPod() *corev1.Pod {
	started := demoEpoch.Add(-25 * time.Minute)
	finished := demoEpoch.Add(-22 * time.Minute)
	controller, block := true, true
	return &corev1.Pod{
		APIVersion: "v1", Kind: "Pod",
		Name:      PodNightlyBackupRun,
		Namespace: NamespaceJobs,
		UID:       k8stypes.UID(uidPodNightlyBackupRun),
		Labels: map[string]string{
			"batch.kubernetes.io/job-name":       JobNightlyBackupRun,
			"batch.kubernetes.io/controller-uid": UIDJobNightlyBackupRun,
		},
		CreationTimestamp: metav1.NewTime(started),
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "batch/v1", Kind: "Job", Name: JobNightlyBackupRun,
				UID: k8stypes.UID(UIDJobNightlyBackupRun), Controller: &controller, BlockOwnerDeletion: &block,
			},
		},
		ManagedFields: []metav1.ManagedFieldsEntry{
			managedField("kube-controller-manager", "Update",
				`{"f:metadata":{"f:ownerReferences":{},"f:labels":{}}}`, started),
			managedField("kubelet", "Update",
				`{"f:status":{"f:containerStatuses":{},"f:phase":{}}}`, finished),
		},
		Spec: corev1.PodSpec{
			NodeName:      NodeWorker1,
			RestartPolicy: corev1.RestartPolicyNever,
			Containers:    []corev1.Container{nightlyBackupContainer()},
		},
		Status: corev1.PodStatus{
			Phase:     corev1.PodSucceeded,
			StartTime: ptrTime(started),
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "backup",
					Ready: false,
					Image: "ghcr.io/example/nightly-backup:1.0.0",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode:   0,
							Reason:     "Completed",
							StartedAt:  metav1.NewTime(started),
							FinishedAt: metav1.NewTime(finished),
						},
					},
				},
			},
		},
	}
}

func buildJob() *batchv1.Job {
	started := demoEpoch.Add(-25 * time.Minute)
	backoffLimit := int32(4)
	return &batchv1.Job{
		APIVersion: "batch/v1", Kind: "Job",
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
		APIVersion: "v1", Kind: "Pod",
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
