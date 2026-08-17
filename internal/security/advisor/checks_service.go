// Service-level advisor checks: StatefulSets whose governing Service is
// missing or not headless, and selector Services with zero or one ready
// endpoint.

package advisor

import (
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"

	"github.com/janosmiko/lfk/internal/security"
)

// serviceFindings runs the Service-related checks.
func (d *clusterData) serviceFindings() []security.Finding {
	return slices.Concat(d.statefulSetServiceFindings(), d.endpointFindings())
}

// statefulSetServiceFindings flags StatefulSets whose spec.serviceName
// references a Service that does not exist or is not headless — pod DNS
// identity (pod-0.svc.ns) silently breaks either way.
func (d *clusterData) statefulSetServiceFindings() []security.Finding {
	if !d.servicesOK {
		return nil
	}
	svcByKey := make(map[string]*corev1.Service, len(d.services))
	for i := range d.services {
		svcByKey[d.services[i].Namespace+"/"+d.services[i].Name] = &d.services[i]
	}
	var out []security.Finding
	for i := range d.workloads {
		w := &d.workloads[i]
		if w.kind != "StatefulSet" || systemNamespaces[w.namespace] {
			continue
		}
		if w.stsServiceName == "" {
			// serviceName is required by validation. An empty value only
			// appears in pre-validation fakes. Nothing to check.
			continue
		}
		svc, ok := svcByKey[w.namespace+"/"+w.stsServiceName]
		switch {
		case !ok:
			out = append(out, makeFinding(w.namespace, w.kind, w.name, "statefulset_service_missing", security.SeverityMedium,
				"governing Service missing",
				fmt.Sprintf("StatefulSet %q names Service %q as its governing service, but it does not exist; per-pod DNS identities will not resolve.", w.name, w.stsServiceName)))
		case svc.Spec.ClusterIP != corev1.ClusterIPNone:
			out = append(out, makeFinding(w.namespace, w.kind, w.name, "statefulset_service_not_headless", security.SeverityMedium,
				"governing Service not headless",
				fmt.Sprintf("StatefulSet %q names Service %q as its governing service, but it is not headless (clusterIP: None); per-pod DNS identities will not resolve.", w.name, w.stsServiceName)))
		}
	}
	return out
}

// endpointFindings flags selector Services with zero ready endpoints (selector matches nothing, or every
// backend is down). It also flags Services with exactly one ready endpoint (any restart is a full outage).
// Selector-less and ExternalName Services manage their endpoints elsewhere and are skipped.
func (d *clusterData) endpointFindings() []security.Finding {
	if !d.servicesOK || !d.epsOK {
		return nil
	}
	ready := d.readyEndpointsByService()
	var out []security.Finding
	for i := range d.services {
		svc := &d.services[i]
		if systemNamespaces[svc.Namespace] || len(svc.Spec.Selector) == 0 ||
			svc.Spec.Type == corev1.ServiceTypeExternalName {
			continue
		}
		switch ready[svc.Namespace+"/"+svc.Name] {
		case 0:
			out = append(out, makeFinding(svc.Namespace, "Service", svc.Name, "service_no_endpoints", security.SeverityMedium,
				"Service has no ready endpoints",
				fmt.Sprintf("Service %q selects no ready pods; either the selector matches nothing or every backend is down. Traffic to it is black-holed.", svc.Name)))
		case 1:
			out = append(out, makeFinding(svc.Namespace, "Service", svc.Name, "single_endpoint_service", security.SeverityLow,
				"Service backed by a single endpoint",
				fmt.Sprintf("Service %q has exactly one ready endpoint; any restart of that pod is a full outage for the service.", svc.Name)))
		}
	}
	return out
}

// readyEndpointsByService counts ready endpoints per "ns/serviceName" from
// EndpointSlices. An endpoint with a nil Ready condition counts as ready,
// per the EndpointSlice API contract ("consumers should interpret unknown
// as true").
func (d *clusterData) readyEndpointsByService() map[string]int {
	ready := make(map[string]int)
	for i := range d.endpointSlices {
		eps := &d.endpointSlices[i]
		svcName := eps.Labels[discoveryv1.LabelServiceName]
		if svcName == "" {
			continue
		}
		key := eps.Namespace + "/" + svcName
		for j := range eps.Endpoints {
			r := eps.Endpoints[j].Conditions.Ready
			if r == nil || *r {
				ready[key]++
			}
		}
	}
	return ready
}
