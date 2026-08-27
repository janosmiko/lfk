package k8s

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// monitoringTarget is one candidate monitoring endpoint reachable through the
// API server proxy. Prefix goes in front of the Prometheus API path, because
// VictoriaMetrics serves that API below a tenant path instead of at the root.
type monitoringTarget struct {
	Namespace string
	Service   string
	Port      string
	Prefix    string
}

// path returns the proxy path for one Prometheus or Alertmanager API path.
func (t monitoringTarget) path(apiPath string) string {
	return t.Prefix + apiPath
}

// Well-known namespaces and service names to probe when discovery finds
// nothing. They cover Prometheus installed by the kube-prometheus-stack chart,
// by the Prometheus operator, and by the upstream prometheus chart.
var (
	defaultMonitoringNamespaces = []string{"monitoring", "prometheus", "observability", "kube-prometheus-stack"}
	defaultPrometheusServices   = []string{
		"kube-prometheus-stack-prometheus", "prometheus-kube-prometheus-prometheus",
		"prometheus-server", "prometheus", "prometheus-operated",
	}
	defaultAlertmanagerServices = []string{
		"alertmanager-operated", "alertmanager",
		"prometheus-kube-prometheus-alertmanager", "alertmanager-main",
	}
)

const (
	defaultPrometheusPort   = "9090"
	defaultAlertmanagerPort = "9093"

	// The VictoriaMetrics operator stamps the component name on every object
	// it creates. The Service name carries the custom resource name as a
	// suffix (vmselect-vmks), so only the label is stable enough to match on.
	appNameLabel = "app.kubernetes.io/name"

	// The Prometheus operator sets these on the headless Service it puts in
	// front of each StatefulSet. kube-prometheus-stack leaves
	// app.kubernetes.io/name off its own Service entirely, so these are the
	// only labels that find a Prometheus on that chart.
	operatedPrometheusLabel   = "operated-prometheus"
	operatedAlertmanagerLabel = "operated-alertmanager"

	// vmselect serves the Prometheus querying API below a tenant path. Account
	// 0 is what the victoria-metrics-k8s-stack chart writes, and the
	// multitenant path covers an install that uses a different account.
	vmSelectTenantPrefix      = "/select/0/prometheus"
	vmSelectMultitenantPrefix = "/select/multitenant/prometheus"
)

// monitoringDiscoveryTTL bounds how long a discovery result is reused. A
// monitoring stack that moves namespace is rare, so a long TTL keeps the
// extra Service list off the metrics poll path.
const monitoringDiscoveryTTL = 10 * time.Minute

type monitoringDiscoveryEntry struct {
	prom []monitoringTarget
	am   []monitoringTarget
	at   time.Time
}

var monitoringDiscoveryCache sync.Map // key: contextName, value: monitoringDiscoveryEntry

// monitoringTargetsFor returns the targets to probe for a cluster context.
// Discovered services come first: each wrong name guess costs a full proxy
// timeout, and a discovered Service is known to exist.
func monitoringTargetsFor(ctx context.Context, cs kubernetes.Interface, contextName string) (prom, am []monitoringTarget) {
	prom, am = resolveMonitoringEndpoints(contextName)
	if !monitoringDiscoveryWanted(contextName) {
		return prom, am
	}
	dProm, dAm := discoveredMonitoringTargets(ctx, cs, contextName)
	return concatTargets(dProm, prom), concatTargets(dAm, am)
}

// discoveredMonitoringTargets returns only what the label search found, with
// no built-in name guesses mixed in. The route decision needs that split: a
// guess is always present, so it cannot tell a caller whether the cluster
// actually has a Prometheus.
func discoveredMonitoringTargets(ctx context.Context, cs kubernetes.Interface, contextName string) (prom, am []monitoringTarget) {
	guessProm, guessAM := resolveMonitoringEndpoints(contextName)
	return cachedMonitoringDiscovery(ctx, cs, contextName, namespacesOf(guessProm, guessAM))
}

// concatTargets joins two target lists into a new slice. Appending onto the
// cached discovery result would write into its backing array, which two
// callers share.
func concatTargets(a, b []monitoringTarget) []monitoringTarget {
	out := make([]monitoringTarget, 0, len(a)+len(b))
	out = append(out, a...)
	return append(out, b...)
}

// monitoringDiscoveryWanted reports whether discovery can still add anything.
// A config that names the services for both roles states the user's intent,
// so lfk probes those names and skips the Service list.
func monitoringDiscoveryWanted(contextName string) bool {
	mc, ok := monitoringConfigFor(contextName)
	if !ok {
		return true
	}
	promNamed := mc.Prometheus != nil && len(mc.Prometheus.Services) > 0
	amNamed := mc.Alertmanager != nil && len(mc.Alertmanager.Services) > 0
	return !promNamed || !amNamed
}

func cachedMonitoringDiscovery(ctx context.Context, cs kubernetes.Interface, contextName string, namespaces []string) (prom, am []monitoringTarget) {
	if cached, ok := monitoringDiscoveryCache.Load(contextName); ok {
		entry := cached.(monitoringDiscoveryEntry)
		if time.Since(entry.at) < monitoringDiscoveryTTL {
			return entry.prom, entry.am
		}
		monitoringDiscoveryCache.Delete(contextName)
	}
	prom, am = discoverMonitoringServices(ctx, cs, namespaces)
	monitoringDiscoveryCache.Store(contextName, monitoringDiscoveryEntry{prom: prom, am: am, at: time.Now()})
	return prom, am
}

// namespacesOf collects the namespaces of the fallback targets, so the
// per-namespace discovery path searches the same places the guesses cover.
func namespacesOf(targetLists ...[]monitoringTarget) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(defaultMonitoringNamespaces))
	for _, targets := range targetLists {
		for _, t := range targets {
			if !seen[t.Namespace] {
				seen[t.Namespace] = true
				out = append(out, t.Namespace)
			}
		}
	}
	return out
}

// monitoringSelectors are the label queries discovery runs. Label selectors
// are ANDed, so one query per label family is the only way to cover both
// operators.
var monitoringSelectors = []string{
	appNameLabel + " in (prometheus,alertmanager,vmsingle,vmselect,vmalertmanager)",
	operatedPrometheusLabel + "=true",
	operatedAlertmanagerLabel + "=true",
}

// discoverMonitoringServices finds monitoring Services by their component
// labels. The cluster-wide list comes first because it also finds a stack
// outside the well-known namespaces, but a user may lack that permission.
func discoverMonitoringServices(ctx context.Context, cs kubernetes.Interface, namespaces []string) (prom, am []monitoringTarget) {
	found := make([]corev1.Service, 0, len(monitoringSelectors))
	for _, selector := range monitoringSelectors {
		found = append(found, listMonitoringServices(ctx, cs, namespaces, selector)...)
	}
	return targetsFromServices(preferWellKnownNamespaces(dedupeServices(found), namespaces))
}

// preferWellKnownNamespaces stably moves Services in the namespaces an
// operator normally owns to the front. Any user who can create a Service can
// carry the labels discovery matches, so the real stack gets probed, cached,
// and answers before one that merely appeared somewhere else.
func preferWellKnownNamespaces(services []corev1.Service, namespaces []string) []corev1.Service {
	known := make(map[string]bool, len(namespaces))
	for _, ns := range namespaces {
		known[ns] = true
	}
	out := make([]corev1.Service, 0, len(services))
	for _, svc := range services {
		if known[svc.Namespace] {
			out = append(out, svc)
		}
	}
	for _, svc := range services {
		if !known[svc.Namespace] {
			out = append(out, svc)
		}
	}
	return out
}

// listMonitoringServices runs one selector, cluster-wide when the user may
// list that wide, and namespace by namespace otherwise.
func listMonitoringServices(ctx context.Context, cs kubernetes.Interface, namespaces []string, selector string) []corev1.Service {
	opts := metav1.ListOptions{LabelSelector: selector}

	list, err := cs.CoreV1().Services(metav1.NamespaceAll).List(ctx, opts)
	if err == nil {
		return list.Items
	}
	logger.Debug("cluster-wide monitoring service discovery failed", "selector", selector, "error", logger.Redact(err.Error()))

	var found []corev1.Service
	for _, ns := range namespaces {
		list, err := cs.CoreV1().Services(ns).List(ctx, opts)
		if err != nil {
			logger.Debug("monitoring service discovery failed", "namespace", ns, "selector", selector, "error", logger.Redact(err.Error()))
			continue
		}
		found = append(found, list.Items...)
	}
	return found
}

// dedupeServices drops repeats, because one Service can carry the labels of
// two selectors.
func dedupeServices(services []corev1.Service) []corev1.Service {
	seen := make(map[string]bool, len(services))
	out := make([]corev1.Service, 0, len(services))
	for _, svc := range services {
		key := svc.Namespace + "/" + svc.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, svc)
	}
	return out
}

// monitoringRole is the half of the monitoring API a discovered Service serves.
type monitoringRole int

const (
	roleNone monitoringRole = iota
	rolePrometheus
	roleAlertmanager
	// roleVMSelect serves the Prometheus API, but below a tenant path.
	roleVMSelect
)

// serviceRole reads the role off a Service's labels. The VictoriaMetrics
// operator names the component in app.kubernetes.io/name. The Prometheus
// operator marks its governing Service instead, which is what a
// kube-prometheus-stack install carries.
func serviceRole(svc *corev1.Service) monitoringRole {
	switch svc.Labels[appNameLabel] {
	case "prometheus", "vmsingle":
		return rolePrometheus
	case "vmselect":
		return roleVMSelect
	case "alertmanager", "vmalertmanager":
		return roleAlertmanager
	}
	switch {
	case svc.Labels[operatedPrometheusLabel] == "true":
		return rolePrometheus
	case svc.Labels[operatedAlertmanagerLabel] == "true":
		return roleAlertmanager
	}
	return roleNone
}

// targetsFromServices turns discovered Services into probe targets. The port
// comes from the Service itself, so a stack on a non-default port still works.
func targetsFromServices(services []corev1.Service) (prom, am []monitoringTarget) {
	for i := range services {
		svc := &services[i]
		p := servicePort(svc)
		if p == "" {
			continue
		}
		base := monitoringTarget{Namespace: svc.Namespace, Service: svc.Name, Port: p}
		switch serviceRole(svc) {
		case rolePrometheus:
			prom = append(prom, base)
		case roleVMSelect:
			prom = append(prom,
				withPrefix(base, vmSelectTenantPrefix),
				withPrefix(base, vmSelectMultitenantPrefix))
		case roleAlertmanager:
			am = append(am, base)
		case roleNone:
		}
	}
	return prom, am
}

func withPrefix(t monitoringTarget, prefix string) monitoringTarget {
	t.Prefix = prefix
	return t
}

// servicePort picks the HTTP port of a monitoring Service. The named port wins
// because an Alertmanager Service also exposes its gossip port, and that one
// answers no API request. Its gossip port carries the same name on UDP as on
// TCP, so the name alone is not enough.
func servicePort(svc *corev1.Service) string {
	for _, p := range svc.Spec.Ports {
		if isTCP(p) && (p.Name == "http" || p.Name == "web") {
			return strconv.Itoa(int(p.Port))
		}
	}
	for _, p := range svc.Spec.Ports {
		if isTCP(p) {
			return strconv.Itoa(int(p.Port))
		}
	}
	return ""
}

func isTCP(p corev1.ServicePort) bool {
	return p.Protocol == "" || p.Protocol == corev1.ProtocolTCP
}

// MonitoringSearchHint says where lfk looked for a monitoring endpoint, for the
// "not reachable" message. It reports each role on its own, because a config
// that names the services for one role leaves the other on the built-in list,
// and a config that names both turns label discovery off entirely.
func MonitoringSearchHint(contextName string) []string {
	mc, _ := monitoringConfigFor(contextName)
	prom, am := resolveMonitoringEndpoints(contextName)

	var lines []string
	if monitoringDiscoveryWanted(contextName) {
		lines = append(lines,
			"Searched the cluster for a Service labelled prometheus,",
			"alertmanager, vmsingle, vmselect, or vmalertmanager.")
	}
	// The namespace list bounds the name probing, not the label search, which
	// covers the whole cluster unless the user cannot list Services that wide.
	return append(lines,
		"Prometheus names: "+endpointNames(mc.Prometheus),
		"Alertmanager names: "+endpointNames(mc.Alertmanager),
		"Probed those names in: "+strings.Join(namespacesOf(prom, am), ", "))
}

func endpointNames(ep *model.MonitoringEndpoint) string {
	if ep == nil || len(ep.Services) == 0 {
		return "the built-in list"
	}
	return strings.Join(ep.Services, ", ")
}

// monitoringConfigFor returns the monitoring config that applies to a cluster
// context: the entry for that context, else the "_global" entry.
func monitoringConfigFor(contextName string) (model.MonitoringConfig, bool) {
	cfg := model.ConfigMonitoring
	if cfg == nil {
		return model.MonitoringConfig{}, false
	}
	if mc, ok := cfg[contextName]; ok {
		return mc, true
	}
	mc, ok := cfg["_global"]
	return mc, ok
}

// resolveMonitoringEndpoints returns the name-guess targets for a cluster
// context: the configured namespaces, services, port, and path prefix if the
// user set them, otherwise the built-in defaults.
func resolveMonitoringEndpoints(contextName string) (prom, am []monitoringTarget) {
	mc, _ := monitoringConfigFor(contextName)
	return endpointTargets(mc.Prometheus, defaultPrometheusServices, defaultPrometheusPort),
		endpointTargets(mc.Alertmanager, defaultAlertmanagerServices, defaultAlertmanagerPort)
}

// endpointTargets expands one endpoint config into the namespace-by-service
// probe list. An unset field keeps its default.
func endpointTargets(ep *model.MonitoringEndpoint, defaultServices []string, defaultPort string) []monitoringTarget {
	namespaces, services, port, prefix := defaultMonitoringNamespaces, defaultServices, defaultPort, ""
	if ep != nil {
		if len(ep.Namespaces) > 0 {
			namespaces = ep.Namespaces
		}
		if len(ep.Services) > 0 {
			services = ep.Services
		}
		if ep.Port != "" {
			port = ep.Port
		}
		prefix = ep.PathPrefix
	}

	targets := make([]monitoringTarget, 0, len(namespaces)*len(services))
	for _, ns := range namespaces {
		for _, svc := range services {
			targets = append(targets, monitoringTarget{Namespace: ns, Service: svc, Port: port, Prefix: prefix})
		}
	}
	return targets
}
