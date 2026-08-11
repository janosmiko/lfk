package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func findUndeliverable(items []UndeliverableItem, ns, name string) *UndeliverableItem {
	for i := range items {
		if items[i].Namespace == ns && items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

// TestDetectUndeliverable_EndToEnd drives every detector through one
// cluster snapshot so the wiring from list call to report slice is covered
// alongside the per-detector unit tests.
func TestDetectUndeliverable_EndToEnd(t *testing.T) {
	now := metav1.Now()
	ready := true

	unschedulable := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "web", Name: "stuck"},
		Status:     corev1.PodStatus{Phase: corev1.PodPending},
	}
	healthy := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "web", Name: "fine"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	schedEvent := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "web", Name: "stuck.1"},
		InvolvedObject: corev1.ObjectReference{Kind: "Pod", Namespace: "web", Name: "stuck"},
		Reason:         "FailedScheduling",
		Message:        "0/1 nodes are available: 1 Insufficient memory.",
		LastTimestamp:  metav1.NewTime(time.Now()),
	}
	unbound := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: "data", Name: "claim"},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
	}
	deadSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "web", Name: "dead"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
	}
	liveSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: "web", Name: "live"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
	}
	liveSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "web", Name: "live-x",
			Labels: map[string]string{discoveryv1.LabelServiceName: "live"},
		},
		Endpoints: []discoveryv1.Endpoint{
			{Conditions: discoveryv1.EndpointConditions{Ready: &ready}},
		},
	}
	pendingIngress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Namespace: "web", Name: "site"},
	}
	stuckNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "doomed", DeletionTimestamp: &now,
			Finalizers: []string{"kubernetes"},
		},
	}

	cs := k8sfake.NewSimpleClientset(
		unschedulable, healthy, schedEvent, unbound,
		deadSvc, liveSvc, liveSlice, pendingIngress, stuckNS,
	)
	c := newFakeClient(cs, nil)

	report, err := c.DetectUndeliverable(t.Context(), "", "")
	require.NoError(t, err)

	require.Len(t, report.Pods, 1)
	assert.Equal(t, "FailedScheduling: 0/1 nodes are available: 1 Insufficient memory.",
		report.Pods[0].Reason)

	require.Len(t, report.PVCs, 1)
	assert.Contains(t, report.PVCs[0].Reason, "no storage class set")

	require.Len(t, report.Services, 1)
	assert.Equal(t, "dead", report.Services[0].Name)

	require.Len(t, report.Ingresses, 1)
	assert.Equal(t, "no address in status.loadBalancer", report.Ingresses[0].Reason)

	require.NotNil(t, findUndeliverable(report.Terminating, "", "doomed"))
	assert.Contains(t, findUndeliverable(report.Terminating, "", "doomed").Reason,
		"blocked by finalizers: kubernetes")
}

// TestDetectUndeliverable_SortStable pins the (namespace, kind, name)
// ordering the overlay relies on for a stable cursor across refetches.
func TestDetectUndeliverable_SortStable(t *testing.T) {
	pods := []*corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "b", Name: "z"},
			Status: corev1.PodStatus{Phase: corev1.PodPending, Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable"},
			}},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Namespace: "a", Name: "y"},
			Status: corev1.PodStatus{Phase: corev1.PodPending, Conditions: []corev1.PodCondition{
				{Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: "Unschedulable"},
			}},
		},
	}
	cs := k8sfake.NewSimpleClientset(pods[0], pods[1])
	c := newFakeClient(cs, nil)

	report, err := c.DetectUndeliverable(t.Context(), "", "")
	require.NoError(t, err)
	require.Len(t, report.Pods, 2)
	assert.Equal(t, "a", report.Pods[0].Namespace)
	assert.Equal(t, "b", report.Pods[1].Namespace)
}
