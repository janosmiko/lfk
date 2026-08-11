package k8s

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// undeliverableLists bundles every API list response DetectUndeliverable
// needs. Pulled into its own type so the detector stays below the gocyclo
// threshold, mirroring orphanLists.
type undeliverableLists struct {
	pods       *corev1.PodList
	events     *corev1.EventList
	pvcs       *corev1.PersistentVolumeClaimList
	services   *corev1.ServiceList
	slices     *discoveryv1.EndpointSliceList
	ingresses  *networkingv1.IngressList
	namespaces *corev1.NamespaceList
}

// fetchUndeliverableLists pulls every input the detectors need. Per-list
// errors accumulate; the corresponding pointer stays nil and the safe*Items
// helpers turn that into an empty slice so a partial RBAC grant still
// produces a usable report.
func fetchUndeliverableLists(
	ctx context.Context, cs kubernetes.Interface, namespace string,
) (undeliverableLists, []error) {
	var errs []error
	collect := func(name string, err error) {
		if err != nil {
			errs = append(errs, fmt.Errorf("listing %s: %w", name, err))
		}
	}
	opts := metav1.ListOptions{}
	out := undeliverableLists{}
	var err error
	out.pods, err = cs.CoreV1().Pods(namespace).List(ctx, opts)
	collect("pods", err)
	out.events, err = cs.CoreV1().Events(namespace).List(ctx, opts)
	collect("events", err)
	out.pvcs, err = cs.CoreV1().PersistentVolumeClaims(namespace).List(ctx, opts)
	collect("pvcs", err)
	out.services, err = cs.CoreV1().Services(namespace).List(ctx, opts)
	collect("services", err)
	out.slices, err = cs.DiscoveryV1().EndpointSlices(namespace).List(ctx, opts)
	collect("endpointslices", err)
	out.ingresses, err = cs.NetworkingV1().Ingresses(namespace).List(ctx, opts)
	collect("ingresses", err)
	// Namespaces are cluster-scoped and a stuck-Terminating namespace is
	// the single most common finalizer deadlock, so list them regardless
	// of the namespace parameter.
	out.namespaces, err = cs.CoreV1().Namespaces().List(ctx, opts)
	collect("namespaces", err)
	return out, errs
}

// DetectUndeliverable scans the cluster (or a single namespace when ns is
// non-empty) for resources that want to reach a state and cannot, and
// explains each one from a first-class field, a status condition, or an
// Event on the object.
//
// Partial RBAC denial is non-fatal: the returned report holds whatever the
// caller's credentials could read, and the error wraps every underlying
// list failure so the UI can render a banner.
func (c *Client) DetectUndeliverable(
	ctx context.Context, kubeContext, namespace string,
) (UndeliverableReport, error) {
	cs, err := c.clientsetForContext(kubeContext)
	if err != nil {
		return UndeliverableReport{}, err
	}

	lists, errs := fetchUndeliverableLists(ctx, cs, namespace)
	events := buildEventIndex(safeEventItems(lists.events))
	sliceIdx := buildEndpointSliceIndex(safeEndpointSliceItems(lists.slices))

	report := UndeliverableReport{}
	pods := safePodItems(lists.pods)
	pvcs := safePVCItems(lists.pvcs)
	services := safeSvcItems(lists.services)
	ingresses := safeIngressItems(lists.ingresses)

	for _, p := range pods {
		if item, ok := pendingPodUndeliverable(p, events); ok {
			report.Pods = append(report.Pods, item)
		}
	}
	for _, p := range pvcs {
		if item, ok := pendingPVCUndeliverable(p, events); ok {
			report.PVCs = append(report.PVCs, item)
		}
	}
	for _, s := range services {
		if item, ok := serviceUndeliverable(s, sliceIdx); ok {
			report.Services = append(report.Services, item)
		}
	}
	for _, i := range ingresses {
		if item, ok := ingressUndeliverable(i); ok {
			report.Ingresses = append(report.Ingresses, item)
		}
	}
	report.Terminating = collectTerminating(lists)

	sortUndeliverable(report.Pods)
	sortUndeliverable(report.PVCs)
	sortUndeliverable(report.Services)
	sortUndeliverable(report.Ingresses)
	sortUndeliverable(report.Terminating)

	return report, errors.Join(errs...)
}

// collectTerminating walks every kind already fetched for the other
// detectors and reports the ones whose deletion is waiting on finalizers.
// Reusing the same lists keeps the scan to one API round of calls.
func collectTerminating(lists undeliverableLists) []UndeliverableItem {
	var out []UndeliverableItem
	add := func(kind string, meta metav1.ObjectMeta) {
		if item, ok := terminatingUndeliverable(kind, meta); ok {
			out = append(out, item)
		}
	}
	for _, o := range safeNamespaceItems(lists.namespaces) {
		add("Namespace", o.ObjectMeta)
	}
	for _, o := range safePodItems(lists.pods) {
		add("Pod", o.ObjectMeta)
	}
	for _, o := range safePVCItems(lists.pvcs) {
		add("PersistentVolumeClaim", o.ObjectMeta)
	}
	for _, o := range safeSvcItems(lists.services) {
		add("Service", o.ObjectMeta)
	}
	for _, o := range safeIngressItems(lists.ingresses) {
		add("Ingress", o.ObjectMeta)
	}
	return out
}

func safeEventItems(l *corev1.EventList) []corev1.Event {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeEndpointSliceItems(l *discoveryv1.EndpointSliceList) []discoveryv1.EndpointSlice {
	if l == nil {
		return nil
	}
	return l.Items
}

func safeNamespaceItems(l *corev1.NamespaceList) []corev1.Namespace {
	if l == nil {
		return nil
	}
	return l.Items
}
