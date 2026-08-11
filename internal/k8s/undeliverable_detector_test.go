package k8s

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testEvent(kind, ns, name, reason, message string, ago time.Duration) corev1.Event {
	return corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: ns, Name: name + ".1"},
		InvolvedObject: corev1.ObjectReference{Kind: kind, Namespace: ns, Name: name},
		Reason:         reason,
		Message:        message,
		LastTimestamp:  metav1.NewTime(time.Now().Add(-ago)),
	}
}

func TestPendingPodUndeliverable(t *testing.T) {
	pending := func(name string, conds ...corev1.PodCondition) corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "web", Name: name},
			Status:     corev1.PodStatus{Phase: corev1.PodPending, Conditions: conds},
		}
	}
	scheduledFalse := corev1.PodCondition{
		Type:    corev1.PodScheduled,
		Status:  corev1.ConditionFalse,
		Reason:  "Unschedulable",
		Message: "0/2 nodes are available: 2 Insufficient cpu.",
	}

	tests := []struct {
		name       string
		pod        corev1.Pod
		events     []corev1.Event
		wantFound  bool
		wantReason string
	}{
		{
			name: "FailedScheduling event wins over the condition",
			pod:  pending("api", scheduledFalse),
			events: []corev1.Event{
				testEvent("Pod", "web", "api", "FailedScheduling",
					"0/3 nodes are available: 3 node(s) had untolerated taint.", time.Minute),
			},
			wantFound:  true,
			wantReason: "FailedScheduling: 0/3 nodes are available: 3 node(s) had untolerated taint.",
		},
		{
			name:       "PodScheduled=False is the fallback when no event survives",
			pod:        pending("api", scheduledFalse),
			wantFound:  true,
			wantReason: "Unschedulable: 0/2 nodes are available: 2 Insufficient cpu.",
		},
		{
			name:      "pending with no first-class explanation is not reported",
			pod:       pending("api"),
			wantFound: false,
		},
		{
			name: "running pod is never undeliverable",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: "web", Name: "api"},
				Status:     corev1.PodStatus{Phase: corev1.PodRunning},
			},
			events: []corev1.Event{
				testEvent("Pod", "web", "api", "FailedScheduling", "stale", time.Hour),
			},
			wantFound: false,
		},
		{
			name: "newest FailedScheduling event is the one reported",
			pod:  pending("api"),
			events: []corev1.Event{
				testEvent("Pod", "web", "api", "FailedScheduling", "older", time.Hour),
				testEvent("Pod", "web", "api", "FailedScheduling", "newer", time.Minute),
			},
			wantFound:  true,
			wantReason: "FailedScheduling: newer",
		},
		{
			name: "event for a different pod does not leak across",
			pod:  pending("api"),
			events: []corev1.Event{
				testEvent("Pod", "web", "other", "FailedScheduling", "not mine", time.Minute),
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := pendingPodUndeliverable(tt.pod, buildEventIndex(tt.events))
			if ok != tt.wantFound {
				t.Fatalf("found = %v, want %v", ok, tt.wantFound)
			}
			if !ok {
				return
			}
			if got.Reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", got.Reason, tt.wantReason)
			}
			if got.Kind != "Pod" || got.Namespace != "web" || got.Name != tt.pod.Name {
				t.Errorf("identity = %+v", got)
			}
		})
	}
}

func TestPendingPVCUndeliverable(t *testing.T) {
	pvc := func(name string, class *string) corev1.PersistentVolumeClaim {
		return corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Namespace: "data", Name: name},
			Spec:       corev1.PersistentVolumeClaimSpec{StorageClassName: class},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
		}
	}
	empty, fast := "", "fast"

	tests := []struct {
		name       string
		pvc        corev1.PersistentVolumeClaim
		events     []corev1.Event
		wantFound  bool
		wantReason string
	}{
		{
			name: "provisioner failure comes from the event",
			pvc:  pvc("cache", &fast),
			events: []corev1.Event{
				testEvent("PersistentVolumeClaim", "data", "cache", "ProvisioningFailed",
					"storageclass.storage.k8s.io \"fast\" not found", time.Minute),
			},
			wantFound:  true,
			wantReason: "ProvisioningFailed: storageclass.storage.k8s.io \"fast\" not found",
		},
		{
			name: "no matching PV comes from the FailedBinding event",
			pvc:  pvc("cache", &fast),
			events: []corev1.Event{
				testEvent("PersistentVolumeClaim", "data", "cache", "FailedBinding",
					"no persistent volumes available for this claim", time.Minute),
			},
			wantFound:  true,
			wantReason: "FailedBinding: no persistent volumes available for this claim",
		},
		{
			name:       "no storage class set is read off the spec",
			pvc:        pvc("cache", &empty),
			wantFound:  true,
			wantReason: "no storage class set: no provisioner, and no PersistentVolume matched",
		},
		{
			name:       "unset storage class is treated the same as empty",
			pvc:        pvc("cache", nil),
			wantFound:  true,
			wantReason: "no storage class set: no provisioner, and no PersistentVolume matched",
		},
		{
			name:      "pending with a class but no event is not reported",
			pvc:       pvc("cache", &fast),
			wantFound: false,
		},
		{
			name: "bound claim is never undeliverable",
			pvc: corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{Namespace: "data", Name: "cache"},
				Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := pendingPVCUndeliverable(tt.pvc, buildEventIndex(tt.events))
			if ok != tt.wantFound {
				t.Fatalf("found = %v, want %v", ok, tt.wantFound)
			}
			if ok && got.Reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", got.Reason, tt.wantReason)
			}
			if ok && got.Kind != "PersistentVolumeClaim" {
				t.Errorf("kind = %q", got.Kind)
			}
		})
	}
}

func TestServiceUndeliverable(t *testing.T) {
	ready := func(v bool) *bool { return &v }
	slice := func(eps ...discoveryv1.Endpoint) discoveryv1.EndpointSlice {
		return discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "web", Name: "api-abc",
				Labels: map[string]string{discoveryv1.LabelServiceName: "api"},
			},
			Endpoints: eps,
		}
	}
	svc := func(t corev1.ServiceType) corev1.Service {
		return corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Namespace: "web", Name: "api"},
			Spec:       corev1.ServiceSpec{Type: t},
		}
	}

	tests := []struct {
		name       string
		svc        corev1.Service
		slices     []discoveryv1.EndpointSlice
		wantFound  bool
		wantReason string
	}{
		{
			name:       "no EndpointSlice at all",
			svc:        svc(corev1.ServiceTypeClusterIP),
			wantFound:  true,
			wantReason: "no EndpointSlice endpoints",
		},
		{
			name: "endpoints present but none ready",
			svc:  svc(corev1.ServiceTypeClusterIP),
			slices: []discoveryv1.EndpointSlice{slice(
				discoveryv1.Endpoint{Conditions: discoveryv1.EndpointConditions{Ready: ready(false)}},
				discoveryv1.Endpoint{Conditions: discoveryv1.EndpointConditions{Ready: ready(false)}},
			)},
			wantFound:  true,
			wantReason: "0 of 2 EndpointSlice endpoints ready",
		},
		{
			name: "one ready endpoint clears the service",
			svc:  svc(corev1.ServiceTypeClusterIP),
			slices: []discoveryv1.EndpointSlice{slice(
				discoveryv1.Endpoint{Conditions: discoveryv1.EndpointConditions{Ready: ready(false)}},
				discoveryv1.Endpoint{Conditions: discoveryv1.EndpointConditions{Ready: ready(true)}},
			)},
			wantFound: false,
		},
		{
			name: "unset Ready means ready",
			svc:  svc(corev1.ServiceTypeClusterIP),
			slices: []discoveryv1.EndpointSlice{slice(
				discoveryv1.Endpoint{},
			)},
			wantFound: false,
		},
		{
			name:      "ExternalName has no endpoints by design",
			svc:       svc(corev1.ServiceTypeExternalName),
			wantFound: false,
		},
		{
			name: "slice belonging to another service is ignored",
			svc:  svc(corev1.ServiceTypeClusterIP),
			slices: []discoveryv1.EndpointSlice{{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "web", Name: "other-abc",
					Labels: map[string]string{discoveryv1.LabelServiceName: "other"},
				},
				Endpoints: []discoveryv1.Endpoint{{}},
			}},
			wantFound:  true,
			wantReason: "no EndpointSlice endpoints",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := serviceUndeliverable(tt.svc, buildEndpointSliceIndex(tt.slices))
			if ok != tt.wantFound {
				t.Fatalf("found = %v, want %v", ok, tt.wantFound)
			}
			if ok && got.Reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", got.Reason, tt.wantReason)
			}
		})
	}
}

func TestIngressUndeliverable(t *testing.T) {
	ing := func(addrs ...networkingv1.IngressLoadBalancerIngress) networkingv1.Ingress {
		return networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Namespace: "web", Name: "site"},
			Status: networkingv1.IngressStatus{
				LoadBalancer: networkingv1.IngressLoadBalancerStatus{Ingress: addrs},
			},
		}
	}

	tests := []struct {
		name       string
		ing        networkingv1.Ingress
		wantFound  bool
		wantReason string
	}{
		{
			name:       "empty status.loadBalancer",
			ing:        ing(),
			wantFound:  true,
			wantReason: "no address in status.loadBalancer",
		},
		{
			name:       "entry with neither IP nor hostname is no address",
			ing:        ing(networkingv1.IngressLoadBalancerIngress{}),
			wantFound:  true,
			wantReason: "no address in status.loadBalancer",
		},
		{
			name:      "IP address clears the ingress",
			ing:       ing(networkingv1.IngressLoadBalancerIngress{IP: "10.0.0.1"}),
			wantFound: false,
		},
		{
			name:      "hostname alone clears the ingress",
			ing:       ing(networkingv1.IngressLoadBalancerIngress{Hostname: "lb.example.com"}),
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ingressUndeliverable(tt.ing)
			if ok != tt.wantFound {
				t.Fatalf("found = %v, want %v", ok, tt.wantFound)
			}
			if ok && got.Reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", got.Reason, tt.wantReason)
			}
		})
	}
}

func TestTerminatingUndeliverable(t *testing.T) {
	now := metav1.Now()

	tests := []struct {
		name       string
		meta       metav1.ObjectMeta
		wantFound  bool
		wantReason string
	}{
		{
			name: "deleting with one finalizer",
			meta: metav1.ObjectMeta{
				Namespace: "data", Name: "cache", DeletionTimestamp: &now,
				Finalizers: []string{"kubernetes.io/pvc-protection"},
			},
			wantFound:  true,
			wantReason: "terminating, blocked by finalizers: kubernetes.io/pvc-protection",
		},
		{
			name: "every blocking finalizer is listed",
			meta: metav1.ObjectMeta{
				Namespace: "data", Name: "cache", DeletionTimestamp: &now,
				Finalizers: []string{"a.example.com/one", "b.example.com/two"},
			},
			wantFound:  true,
			wantReason: "terminating, blocked by finalizers: a.example.com/one, b.example.com/two",
		},
		{
			name: "deleting with no finalizer is not blocked",
			meta: metav1.ObjectMeta{
				Namespace: "data", Name: "cache", DeletionTimestamp: &now,
			},
			wantFound: false,
		},
		{
			name: "finalizers without a deletionTimestamp are not blocking anything",
			meta: metav1.ObjectMeta{
				Namespace: "data", Name: "cache",
				Finalizers: []string{"kubernetes.io/pvc-protection"},
			},
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := terminatingUndeliverable("PersistentVolumeClaim", tt.meta)
			if ok != tt.wantFound {
				t.Fatalf("found = %v, want %v", ok, tt.wantFound)
			}
			if ok && got.Reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", got.Reason, tt.wantReason)
			}
			if ok && (got.Kind != "PersistentVolumeClaim" || got.Name != "cache") {
				t.Errorf("identity = %+v", got)
			}
		})
	}
}

func TestUndeliverableReportAll(t *testing.T) {
	r := UndeliverableReport{
		Pods:        []UndeliverableItem{{Kind: "Pod", Namespace: "web", Name: "api"}},
		Services:    []UndeliverableItem{{Kind: "Service", Namespace: "web", Name: "api"}},
		Terminating: []UndeliverableItem{{Kind: "Namespace", Name: "old"}},
	}
	if got := len(r.All()); got != 3 {
		t.Fatalf("All() = %d items, want 3", got)
	}
}
