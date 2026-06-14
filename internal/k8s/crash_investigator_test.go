package k8s

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestGetCrashInvestigation_PodSummary(t *testing.T) {
	created := time.Now().Add(-5 * time.Minute)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "crashy",
			Namespace:         "default",
			CreationTimestamp: metav1.Time{Time: created},
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ReplicaSet", Name: "rs-1", Controller: new(true)},
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
			Containers: []corev1.Container{
				{Name: "app", Image: "busybox:1.36"},
			},
		},
		Status: corev1.PodStatus{
			Phase:    corev1.PodRunning,
			PodIP:    "10.0.0.5",
			QOSClass: corev1.PodQOSBurstable,
		},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)

	c.describeOverride = func(_ context.Context, _, _, _ string) (string, error) {
		return "Name: crashy\nNamespace: default\n", nil
	}

	got, err := c.GetCrashInvestigation(t.Context(), "", "default", "crashy")
	require.NoError(t, err)
	assert.Equal(t, "crashy", got.Pod.Name)
	assert.Equal(t, "default", got.Pod.Namespace)
	assert.Equal(t, "Running", got.Pod.Phase)
	assert.Equal(t, "10.0.0.5", got.Pod.PodIP)
	assert.Equal(t, "node-1", got.Pod.Node)
	assert.Equal(t, "Burstable", got.Pod.QoSClass)
	assert.Equal(t, "ReplicaSet", got.Pod.OwnerKind)
	assert.Equal(t, "rs-1", got.Pod.OwnerName)
	assert.Greater(t, got.Pod.Age, 4*time.Minute)
}

func TestGetCrashInvestigation_SingleContainerCLB(t *testing.T) {
	now := time.Now()
	started := now.Add(-30 * time.Second)
	finished := now.Add(-5 * time.Second)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "busybox:1.36"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "app",
					Image:        "busybox:1.36",
					Ready:        false,
					RestartCount: 7,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "CrashLoopBackOff",
							Message: "back-off 5m0s restarting failed container",
						},
					},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Reason:     "Error",
							ExitCode:   1,
							Signal:     0,
							StartedAt:  metav1.Time{Time: started},
							FinishedAt: metav1.Time{Time: finished},
							Message:    "boom",
						},
					},
				},
			},
		},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)
	c.describeOverride = func(_ context.Context, _, _, _ string) (string, error) { return "", nil }

	got, err := c.GetCrashInvestigation(t.Context(), "", "default", "p")
	require.NoError(t, err)
	require.Len(t, got.AppContainers, 1)
	require.Empty(t, got.InitContainers)

	cc := got.AppContainers[0]
	assert.Equal(t, "app", cc.Name)
	assert.Equal(t, "busybox:1.36", cc.Image)
	assert.False(t, cc.IsInit)
	assert.Equal(t, "Waiting", cc.State)
	assert.Equal(t, "CrashLoopBackOff", cc.StateReason)
	assert.False(t, cc.Ready)
	assert.Equal(t, int32(7), cc.RestartCount)
	require.NotNil(t, cc.LastTermination)
	assert.Equal(t, "Error", cc.LastTermination.Reason)
	assert.Equal(t, int32(1), cc.LastTermination.ExitCode)
	assert.Equal(t, "boom", cc.LastTermination.Message)
	assert.Equal(t, started.Unix(), cc.LastTermination.StartedAt.Unix())
}

func TestGetCrashInvestigation_InitContainerCLB(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "init-db", Image: "busybox"},
			},
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "init-db",
					Ready:        false,
					RestartCount: 4,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
					},
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "app", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"}}},
			},
		},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)
	c.describeOverride = func(_ context.Context, _, _, _ string) (string, error) { return "", nil }

	got, err := c.GetCrashInvestigation(t.Context(), "", "default", "p")
	require.NoError(t, err)
	require.Len(t, got.InitContainers, 1)
	require.Len(t, got.AppContainers, 1)
	assert.Equal(t, "init-db", got.InitContainers[0].Name)
	assert.True(t, got.InitContainers[0].IsInit)
	assert.Equal(t, "CrashLoopBackOff", got.InitContainers[0].StateReason)
	assert.False(t, got.AppContainers[0].IsInit)
}

func TestGetCrashInvestigation_MultiContainerOnlyOneFailing(t *testing.T) {
	now := time.Now()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx"},
				{Name: "sidecar", Image: "busybox"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "app", Ready: true, RestartCount: 0,
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.Time{Time: now.Add(-time.Minute)}}},
				},
				{
					Name: "sidecar", Ready: false, RestartCount: 3,
					State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
					LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 1}},
				},
			},
		},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)
	c.describeOverride = func(_ context.Context, _, _, _ string) (string, error) { return "", nil }

	got, err := c.GetCrashInvestigation(t.Context(), "", "default", "p")
	require.NoError(t, err)
	require.Len(t, got.AppContainers, 2)
	assert.Equal(t, "app", got.AppContainers[0].Name)
	assert.Nil(t, got.AppContainers[0].LastTermination)
	assert.Equal(t, "sidecar", got.AppContainers[1].Name)
	require.NotNil(t, got.AppContainers[1].LastTermination)
	assert.Equal(t, int32(3), got.AppContainers[1].RestartCount)
}

func TestGetCrashInvestigation_HealthyPod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx"}}},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "app", Ready: true, RestartCount: 0,
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
			},
		},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)
	c.describeOverride = func(_ context.Context, _, _, _ string) (string, error) { return "", nil }

	got, err := c.GetCrashInvestigation(t.Context(), "", "default", "p")
	require.NoError(t, err)
	assert.Len(t, got.AppContainers, 1)
	assert.True(t, got.AppContainers[0].Ready)
	assert.Equal(t, int32(0), got.AppContainers[0].RestartCount)
	assert.Nil(t, got.AppContainers[0].LastTermination)
}

func TestGetCrashInvestigation_LogsPopulated(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:                 "app",
					RestartCount:         1,
					State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
					LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 1}},
				},
			},
		},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)
	c.describeOverride = func(_ context.Context, _, _, _ string) (string, error) { return "", nil }

	// fakeclient `GetLogs` returns "fake logs" by default; that's enough
	// to assert both PreviousLog and CurrentLog are populated.
	got, err := c.GetCrashInvestigation(t.Context(), "", "default", "p")
	require.NoError(t, err)
	require.Len(t, got.AppContainers, 1)
	cc := got.AppContainers[0]
	assert.NotEmpty(t, cc.PreviousLog, "previous log must be populated for fake clientset")
	assert.NotEmpty(t, cc.CurrentLog, "current log must be populated for fake clientset")
	assert.Empty(t, cc.LogError)
}

func TestGetCrashInvestigation_EventsFiltered(t *testing.T) {
	now := time.Now()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	}
	mine := corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "ev1", Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "p", Namespace: "default"},
		Type:           corev1.EventTypeWarning,
		Reason:         "BackOff",
		Message:        "Back-off restarting failed container",
		LastTimestamp:  metav1.Time{Time: now},
	}
	older := corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "ev2", Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "p", Namespace: "default"},
		Type:           corev1.EventTypeNormal,
		Reason:         "Pulled",
		Message:        "Image pulled",
		LastTimestamp:  metav1.Time{Time: now.Add(-1 * time.Minute)},
	}
	other := corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Name: "ev3", Namespace: "default"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "other-pod", Namespace: "default"},
		Type:           corev1.EventTypeNormal,
		LastTimestamp:  metav1.Time{Time: now},
	}
	cs := k8sfake.NewClientset(pod, &mine, &older, &other)
	c := newFakeClient(cs, nil)
	c.describeOverride = func(_ context.Context, _, _, _ string) (string, error) { return "", nil }

	got, err := c.GetCrashInvestigation(t.Context(), "", "default", "p")
	require.NoError(t, err)
	require.Len(t, got.Events, 2)
	// Newest first.
	assert.Equal(t, "ev1", got.Events[0].Name)
	assert.Equal(t, "ev2", got.Events[1].Name)
}

func TestGetCrashInvestigation_PodNotFound(t *testing.T) {
	cs := k8sfake.NewClientset()
	c := newFakeClient(cs, nil)
	_, err := c.GetCrashInvestigation(t.Context(), "", "default", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetCrashInvestigation_DescribeFailureNonFatal(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
	}
	cs := k8sfake.NewClientset(pod)
	c := newFakeClient(cs, nil)
	c.describeOverride = func(_ context.Context, _, _, _ string) (string, error) {
		return "", fmt.Errorf("kubectl not in PATH")
	}

	got, err := c.GetCrashInvestigation(t.Context(), "", "default", "p")
	require.NoError(t, err, "describe failure must not fail the whole call")
	require.NotNil(t, got)
	assert.Empty(t, got.Describe)
	assert.Contains(t, got.DescribeError, "kubectl not in PATH")
}
