package demo

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
)

func buildEvents() []*corev1.Event {
	crashFirst := demoEpoch.Add(-35 * time.Minute)
	crashLast := demoEpoch.Add(-3 * time.Minute)
	return []*corev1.Event{
		podWarningEvent("web-crash-backoff", uidEventPodCrashBackOff, PodWebCrashLoop, uidPodWebCrash,
			"BackOff", "Back-off restarting failed container web in pod "+PodWebCrashLoop+"_"+NamespaceDemo+
				"("+uidPodWebCrash+")",
			12, crashFirst, crashLast),
		podWarningEvent("web-crash-unhealthy", uidEventPodCrashUnhealthy, PodWebCrashLoop, uidPodWebCrash,
			"Unhealthy", "Readiness probe failed: dial tcp 10.0.2.13:8080: connect: connection refused",
			8, crashFirst.Add(2*time.Minute), crashLast.Add(-30*time.Second)),
		jobWarningEvent(),
	}
}

func podWarningEvent(name, uid, podName, podUID, reason, message string, count int32, first, last time.Time) *corev1.Event {
	return &corev1.Event{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Event"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         NamespaceDemo,
			UID:               k8stypes.UID(uid),
			CreationTimestamp: metav1.NewTime(last),
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod", Name: podName, Namespace: NamespaceDemo, UID: k8stypes.UID(podUID),
		},
		Reason:         reason,
		Message:        message,
		Type:           corev1.EventTypeWarning,
		Count:          count,
		Source:         corev1.EventSource{Component: "kubelet", Host: NodeWorker1},
		FirstTimestamp: metav1.NewTime(first),
		LastTimestamp:  metav1.NewTime(last),
	}
}

func jobWarningEvent() *corev1.Event {
	at := demoEpoch.Add(-2 * time.Minute)
	return &corev1.Event{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Event"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              JobDBMigrate + ".backofflimit",
			Namespace:         NamespaceJobs,
			UID:               k8stypes.UID(uidEventJobBackoffLimit),
			CreationTimestamp: metav1.NewTime(at),
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Job", Name: JobDBMigrate, Namespace: NamespaceJobs, UID: k8stypes.UID(uidJobDBMigrate),
		},
		Reason:         "BackoffLimitExceeded",
		Message:        "Job has reached the specified backoff limit",
		Type:           corev1.EventTypeWarning,
		Count:          1,
		Source:         corev1.EventSource{Component: "job-controller"},
		FirstTimestamp: metav1.NewTime(at),
		LastTimestamp:  metav1.NewTime(at),
	}
}
