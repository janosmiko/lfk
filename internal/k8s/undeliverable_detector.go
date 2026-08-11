package k8s

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UndeliverableItem is one resource that wants to reach a state and
// cannot. Reason always originates from a first-class API field, a status
// condition, or an Event on the object itself. A resource whose blockage
// cannot be explained from one of those three sources is left out
// entirely - a guessed reason is worse than a missing row.
type UndeliverableItem struct {
	Namespace string
	Name      string
	Kind      string
	Reason    string
}

// UndeliverableReport groups stuck resources by the detector that found
// them. Each slice is sorted by (namespace, name) so renders are stable
// across fetches.
type UndeliverableReport struct {
	Pods        []UndeliverableItem // Pending, cannot be scheduled
	PVCs        []UndeliverableItem // Pending, cannot be bound
	Services    []UndeliverableItem // no ready endpoint to deliver traffic to
	Ingresses   []UndeliverableItem // no address published in status.loadBalancer
	Terminating []UndeliverableItem // deletionTimestamp set, finalizers pending
}

// All returns every item in the report in kind order.
func (r UndeliverableReport) All() []UndeliverableItem {
	out := make([]UndeliverableItem, 0,
		len(r.Pods)+len(r.PVCs)+len(r.Services)+len(r.Ingresses)+len(r.Terminating))
	out = append(out, r.Pods...)
	out = append(out, r.PVCs...)
	out = append(out, r.Services...)
	out = append(out, r.Ingresses...)
	out = append(out, r.Terminating...)
	return out
}

// eventKey identifies the object an Event is about.
type eventKey struct {
	kind      string
	namespace string
	name      string
}

// eventIndex groups Events by the object they involve so a detector can
// look up its own object's events without rescanning the whole list per
// resource.
type eventIndex map[eventKey][]corev1.Event

func buildEventIndex(events []corev1.Event) eventIndex {
	idx := make(eventIndex, len(events))
	for _, e := range events {
		k := eventKey{
			kind:      e.InvolvedObject.Kind,
			namespace: e.InvolvedObject.Namespace,
			name:      e.InvolvedObject.Name,
		}
		idx[k] = append(idx[k], e)
	}
	return idx
}

// eventTime is the most recent timestamp an Event carries. The three
// fields are populated by different apiserver versions and event
// recorders, so picking the newest of them is the only ordering that
// works across clusters.
func eventTime(e corev1.Event) time.Time {
	newest := e.CreationTimestamp.Time
	if e.LastTimestamp.After(newest) {
		newest = e.LastTimestamp.Time
	}
	if e.EventTime.After(newest) {
		newest = e.EventTime.Time
	}
	return newest
}

// latestReason returns "Reason: Message" from the newest Event on the
// named object whose Reason is one of `reasons`. Both halves are verbatim
// apiserver data, which is what makes the row trustworthy.
func (idx eventIndex) latestReason(kind, namespace, name string, reasons ...string) (string, bool) {
	events := idx[eventKey{kind: kind, namespace: namespace, name: name}]
	var best *corev1.Event
	for i := range events {
		if !slices.Contains(reasons, events[i].Reason) {
			continue
		}
		if best == nil || eventTime(events[i]).After(eventTime(*best)) {
			best = &events[i]
		}
	}
	if best == nil {
		return "", false
	}
	return joinReason(best.Reason, best.Message), true
}

// joinReason formats a reason/message pair, tolerating either half being
// empty - a recorder is free to omit the message.
func joinReason(reason, message string) string {
	switch {
	case reason == "":
		return message
	case message == "":
		return reason
	default:
		return reason + ": " + message
	}
}

// podFailedScheduling is the Event reason the scheduler records when it
// cannot place a Pod.
const podFailedScheduling = "FailedScheduling"

// pendingPodUndeliverable reports a Pending Pod that the scheduler has
// said it cannot place. The FailedScheduling Event is preferred because
// it carries the per-node breakdown; the PodScheduled=False condition is
// the fallback for clusters whose events have already expired.
func pendingPodUndeliverable(pod corev1.Pod, idx eventIndex) (UndeliverableItem, bool) {
	if pod.Status.Phase != corev1.PodPending {
		return UndeliverableItem{}, false
	}
	item := UndeliverableItem{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name}
	if reason, ok := idx.latestReason("Pod", pod.Namespace, pod.Name, podFailedScheduling); ok {
		item.Reason = reason
		return item, true
	}
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse {
			item.Reason = joinReason(c.Reason, c.Message)
			return item, true
		}
	}
	return UndeliverableItem{}, false
}

// pvcBindingEventReasons are the Event reasons the PV controller and
// external provisioners record for a claim that cannot bind.
var pvcBindingEventReasons = []string{
	"ProvisioningFailed",
	"FailedBinding",
	"WaitForFirstConsumer",
	"WaitForPodScheduled",
	"ExternalProvisioning",
}

// pendingPVCUndeliverable reports a Pending PersistentVolumeClaim.
//
// A claim with no storage class is explained from the spec alone: without
// one there is no provisioner, so binding depends on a pre-existing PV
// and Pending means none matched. Every other Pending claim needs an
// Event to say why, because "which provisioner is stuck and how" is not
// derivable from the claim.
func pendingPVCUndeliverable(pvc corev1.PersistentVolumeClaim, idx eventIndex) (UndeliverableItem, bool) {
	if pvc.Status.Phase != corev1.ClaimPending {
		return UndeliverableItem{}, false
	}
	item := UndeliverableItem{
		Kind: "PersistentVolumeClaim", Namespace: pvc.Namespace, Name: pvc.Name,
	}
	reason, ok := idx.latestReason(
		"PersistentVolumeClaim", pvc.Namespace, pvc.Name, pvcBindingEventReasons...)
	if ok {
		item.Reason = reason
		return item, true
	}
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName == "" {
		item.Reason = "no storage class set: no provisioner, and no PersistentVolume matched"
		return item, true
	}
	return UndeliverableItem{}, false
}

// nsName identifies a namespaced object without its kind.
type nsName struct {
	namespace string
	name      string
}

// endpointSliceIndex groups EndpointSlices by the Service they back,
// read off the kubernetes.io/service-name label the slice controller sets.
type endpointSliceIndex map[nsName][]discoveryv1.EndpointSlice

func buildEndpointSliceIndex(list []discoveryv1.EndpointSlice) endpointSliceIndex {
	idx := make(endpointSliceIndex, len(list))
	for _, s := range list {
		svc := s.Labels[discoveryv1.LabelServiceName]
		if svc == "" {
			continue
		}
		k := nsName{namespace: s.Namespace, name: svc}
		idx[k] = append(idx[k], s)
	}
	return idx
}

// serviceUndeliverable reports a Service with no ready endpoint, so
// traffic sent to it has nowhere to land. ExternalName Services are
// excluded: they resolve via DNS and never have endpoints.
func serviceUndeliverable(svc corev1.Service, idx endpointSliceIndex) (UndeliverableItem, bool) {
	if svc.Spec.Type == corev1.ServiceTypeExternalName {
		return UndeliverableItem{}, false
	}
	total, ready := 0, 0
	for _, s := range idx[nsName{namespace: svc.Namespace, name: svc.Name}] {
		for _, ep := range s.Endpoints {
			total++
			// An unset Ready condition means ready - see the
			// EndpointConditions API contract.
			if ep.Conditions.Ready == nil || *ep.Conditions.Ready {
				ready++
			}
		}
	}
	if ready > 0 {
		return UndeliverableItem{}, false
	}
	item := UndeliverableItem{Kind: "Service", Namespace: svc.Namespace, Name: svc.Name}
	if total == 0 {
		item.Reason = "no EndpointSlice endpoints"
	} else {
		item.Reason = fmt.Sprintf("0 of %d EndpointSlice endpoints ready", total)
	}
	return item, true
}

// ingressUndeliverable reports an Ingress whose controller has not
// published a reachable address, so no external traffic can arrive.
func ingressUndeliverable(ing networkingv1.Ingress) (UndeliverableItem, bool) {
	for _, addr := range ing.Status.LoadBalancer.Ingress {
		if addr.IP != "" || addr.Hostname != "" {
			return UndeliverableItem{}, false
		}
	}
	return UndeliverableItem{
		Kind:      "Ingress",
		Namespace: ing.Namespace,
		Name:      ing.Name,
		Reason:    "no address in status.loadBalancer",
	}, true
}

// terminatingUndeliverable reports an object whose deletion is waiting on
// finalizers. Without a finalizer there is nothing to name as the
// blocker, so those objects are skipped - they are mid-teardown, not stuck.
func terminatingUndeliverable(kind string, meta metav1.ObjectMeta) (UndeliverableItem, bool) {
	if meta.DeletionTimestamp == nil || len(meta.Finalizers) == 0 {
		return UndeliverableItem{}, false
	}
	return UndeliverableItem{
		Kind:      kind,
		Namespace: meta.Namespace,
		Name:      meta.Name,
		Reason:    "terminating, blocked by finalizers: " + strings.Join(meta.Finalizers, ", "),
	}, true
}

func sortUndeliverable(items []UndeliverableItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Namespace != items[j].Namespace {
			return items[i].Namespace < items[j].Namespace
		}
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Name < items[j].Name
	})
}
