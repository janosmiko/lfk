package k8s

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

// populatePodDetails extracts ready count, restarts, container status,
// resource requests/limits, and additional metadata columns for a Pod.
func populatePodDetails(ti *model.Item, obj map[string]any, status, spec map[string]any) {
	if status == nil {
		return
	}
	containerStatuses, _ := status["containerStatuses"].([]any)
	totalContainers := len(containerStatuses)
	if containers, ok := spec["containers"].([]any); ok {
		totalContainers = len(containers)
	}
	readyCount := 0
	restartCount := int64(0)
	for _, cs := range containerStatuses {
		csMap, ok := cs.(map[string]any)
		if !ok {
			continue
		}
		if ready, ok := csMap["ready"].(bool); ok && ready {
			readyCount++
		}
		if rc, ok := csMap["restartCount"].(int64); ok {
			restartCount += rc
		} else if rcf, ok := csMap["restartCount"].(float64); ok {
			restartCount += int64(rcf)
		}
	}
	ti.Ready = fmt.Sprintf("%d/%d", readyCount, totalContainers)
	ti.Restarts = fmt.Sprintf("%d", restartCount)

	ti.LastRestartAt = findLastRestartTime(containerStatuses)

	// Override status based on container readiness.
	// Succeeded pods stay green even with unready containers.
	if ti.Status != "Succeeded" && readyCount < totalContainers && totalContainers > 0 {
		overridePodStatus(ti, status, containerStatuses)
	}

	// Resource requests/limits from container specs.
	if containers, ok := spec["containers"].([]any); ok {
		cpuReq, cpuLim, memReq, memLim := extractContainerResources(containers)
		addResourceColumns(ti, cpuReq, cpuLim, memReq, memLim)
	}

	populatePodExtraColumns(ti, obj, status, spec)
}

// findLastRestartTime finds the most recent restart time from container lastState.
func findLastRestartTime(containerStatuses []any) time.Time {
	var lastRestart time.Time
	for _, cs := range containerStatuses {
		csMap, ok := cs.(map[string]any)
		if !ok {
			continue
		}
		lastState, _ := csMap["lastState"].(map[string]any)
		if lastState == nil {
			continue
		}
		if terminated, ok := lastState["terminated"].(map[string]any); ok {
			if finishedAt, ok := terminated["finishedAt"].(string); ok {
				if t, err := time.Parse(time.RFC3339, finishedAt); err == nil {
					if t.After(lastRestart) {
						lastRestart = t
					}
				}
			}
		}
	}
	return lastRestart
}

// overridePodStatus sets the pod status based on init/container readiness reasons.
func overridePodStatus(ti *model.Item, status map[string]any, containerStatuses []any) {
	// Check init container statuses first -- when an init container fails,
	// regular containers show "PodInitializing" which hides the real reason.
	initContainerStatuses, _ := status["initContainerStatuses"].([]any)
	reason := extractContainerNotReadyReason(initContainerStatuses)
	if reason == "" || reason == "PodInitializing" {
		reason = extractContainerNotReadyReason(containerStatuses)
	}
	// If the pod phase is Failed, prefer that over "PodInitializing".
	if reason == "PodInitializing" && ti.Status == "Failed" {
		reason = ""
	}
	if reason != "" {
		ti.Status = reason
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reason", Value: reason})
	} else if ti.Status == "Running" {
		ti.Status = "NotReady"
	}
}

// populatePodExtraColumns adds QoS, service account, images, priority class,
// and node columns to a Pod item.
func populatePodExtraColumns(ti *model.Item, _ map[string]any, status, spec map[string]any) {
	if qos, ok := status["qosClass"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "QoS", Value: qos})
	}
	if sa, ok := spec["serviceAccountName"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Service Account", Value: sa})
	}
	if podIP, ok := status["podIP"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Pod IP", Value: podIP})
	}
	if containers, ok := spec["containers"].([]any); ok {
		var images []string
		for _, c := range containers {
			if cMap, ok := c.(map[string]any); ok {
				if img, ok := cMap["image"].(string); ok {
					images = append(images, img)
				}
			}
		}
		if len(images) > 0 {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Images", Value: strings.Join(images, ", ")})
		}
	}
	// Priority class.
	if pc, ok := spec["priorityClassName"].(string); ok && pc != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Priority Class", Value: pc})
	}
	// Node at the end (lower priority in table view).
	if nodeName, ok := spec["nodeName"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Node", Value: nodeName})
	}
}

// populateDeploymentDetails extracts replica counts, strategy, and resource info for a Deployment.
func populateDeploymentDetails(ti *model.Item, status, spec map[string]any) {
	if status == nil || spec == nil {
		return
	}
	var specReplicas int64 = 1
	if r, ok := spec["replicas"].(int64); ok {
		specReplicas = r
	} else if r, ok := spec["replicas"].(float64); ok {
		specReplicas = int64(r)
	}
	var readyReplicas int64
	if r, ok := status["readyReplicas"].(int64); ok {
		readyReplicas = r
	} else if r, ok := status["readyReplicas"].(float64); ok {
		readyReplicas = int64(r)
	}
	ti.Ready = fmt.Sprintf("%d/%d", readyReplicas, specReplicas)
	// Additional columns.
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Replicas", Value: fmt.Sprintf("%d", specReplicas)})
	if strategy, ok := spec["strategy"].(map[string]any); ok {
		if t, ok := strategy["type"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Strategy", Value: t})
		}
	}
	if updated, ok := status["updatedReplicas"].(float64); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Up-to-date", Value: fmt.Sprintf("%d", int64(updated))})
	}
	if avail, ok := status["availableReplicas"].(float64); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Available", Value: fmt.Sprintf("%d", int64(avail))})
	}
	// Aggregated resource requests/limits (per-pod from template).
	cpuReq, cpuLim, memReq, memLim := extractTemplateResources(spec)
	addResourceColumns(ti, cpuReq, cpuLim, memReq, memLim)
	populateContainerImages(ti, spec)
}

// populateStatefulSetDetails extracts replica counts and resource info for a StatefulSet.
func populateStatefulSetDetails(ti *model.Item, status, spec map[string]any) {
	if status == nil || spec == nil {
		return
	}
	var specReplicas int64 = 1
	if r, ok := spec["replicas"].(int64); ok {
		specReplicas = r
	} else if r, ok := spec["replicas"].(float64); ok {
		specReplicas = int64(r)
	}
	var readyReplicas int64
	if r, ok := status["readyReplicas"].(int64); ok {
		readyReplicas = r
	} else if r, ok := status["readyReplicas"].(float64); ok {
		readyReplicas = int64(r)
	}
	ti.Ready = fmt.Sprintf("%d/%d", readyReplicas, specReplicas)
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Replicas", Value: fmt.Sprintf("%d", specReplicas)})
	// Aggregated resource requests/limits (per-pod from template).
	cpuReq, cpuLim, memReq, memLim := extractTemplateResources(spec)
	addResourceColumns(ti, cpuReq, cpuLim, memReq, memLim)
	populateContainerImages(ti, spec)
}

// populateDaemonSetDetails extracts desired/ready counts and resource info for a DaemonSet.
func populateDaemonSetDetails(ti *model.Item, status, spec map[string]any) {
	if status == nil {
		return
	}
	var desired, ready int64
	if d, ok := status["desiredNumberScheduled"].(int64); ok {
		desired = d
	} else if d, ok := status["desiredNumberScheduled"].(float64); ok {
		desired = int64(d)
	}
	if r, ok := status["numberReady"].(int64); ok {
		ready = r
	} else if r, ok := status["numberReady"].(float64); ok {
		ready = int64(r)
	}
	ti.Ready = fmt.Sprintf("%d/%d", ready, desired)
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Desired", Value: fmt.Sprintf("%d", desired)})
	// Per-pod resource requests/limits from template.
	if spec != nil {
		cpuReq, cpuLim, memReq, memLim := extractTemplateResources(spec)
		addResourceColumns(ti, cpuReq, cpuLim, memReq, memLim)
	}
}

// populateReplicaSetDetails extracts replica counts for a ReplicaSet.
func populateReplicaSetDetails(ti *model.Item, status, spec map[string]any) {
	if status == nil || spec == nil {
		return
	}
	var specReplicas int64
	if r, ok := spec["replicas"].(int64); ok {
		specReplicas = r
	} else if r, ok := spec["replicas"].(float64); ok {
		specReplicas = int64(r)
	}
	var readyReplicas int64
	if r, ok := status["readyReplicas"].(int64); ok {
		readyReplicas = r
	} else if r, ok := status["readyReplicas"].(float64); ok {
		readyReplicas = int64(r)
	}
	ti.Ready = fmt.Sprintf("%d/%d", readyReplicas, specReplicas)
}

// populateServiceDetails extracts type, cluster IP, ports, external IPs,
// load balancer addresses, selector, and session affinity for a Service.
func populateServiceDetails(ti *model.Item, status, spec map[string]any) {
	if spec == nil {
		return
	}
	if svcType, ok := spec["type"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Type", Value: svcType})
	}
	if clusterIP, ok := spec["clusterIP"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Cluster IP", Value: clusterIP})
	}
	populateServicePorts(ti, spec)
	populateServiceExternalIPs(ti, spec)
	populateLoadBalancerAddresses(ti, status, "External Address")
	populateServiceSelector(ti, spec)
	if spec["sessionAffinity"] != nil {
		if sa, ok := spec["sessionAffinity"].(string); ok && sa != "None" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Session Affinity", Value: sa})
		}
	}
}

// populateServicePorts extracts port information from a Service spec.
// Format mirrors kubectl when nodePort is set ("port:nodePort/protocol"),
// while preserving lfk's targetPort visibility ("port→targetPort/protocol",
// or combined: "port:nodePort→targetPort/protocol").
func populateServicePorts(ti *model.Item, spec map[string]any) {
	ports, ok := spec["ports"].([]any)
	if !ok {
		return
	}
	var portStrs []string
	for _, p := range ports {
		if pMap, ok := p.(map[string]any); ok {
			port := getInt(pMap, "port")
			nodePort := getInt(pMap, "nodePort")
			targetPort := getInt(pMap, "targetPort")
			proto, _ := pMap["protocol"].(string)
			head := fmt.Sprintf("%d", port)
			if nodePort > 0 {
				head = fmt.Sprintf("%d:%d", port, nodePort)
			}
			s := fmt.Sprintf("%s/%s", head, proto)
			if targetPort > 0 && targetPort != port {
				s = fmt.Sprintf("%s→%d/%s", head, targetPort, proto)
			}
			portStrs = append(portStrs, s)
		}
	}
	if len(portStrs) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ports", Value: strings.Join(portStrs, ", ")})
	}
}

// populateServiceExternalIPs extracts external IPs from a Service spec.
func populateServiceExternalIPs(ti *model.Item, spec map[string]any) {
	extIPs, ok := spec["externalIPs"].([]any)
	if !ok || len(extIPs) == 0 {
		return
	}
	var ips []string
	for _, ip := range extIPs {
		if s, ok := ip.(string); ok {
			ips = append(ips, s)
		}
	}
	if len(ips) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "External IPs", Value: strings.Join(ips, ", ")})
	}
}

// populateLoadBalancerAddresses extracts IP/hostname addresses from
// status.loadBalancer.ingress and appends them with the given column key.
func populateLoadBalancerAddresses(ti *model.Item, status map[string]any, columnKey string) {
	if status == nil {
		return
	}
	lb, ok := status["loadBalancer"].(map[string]any)
	if !ok {
		return
	}
	ingress, ok := lb["ingress"].([]any)
	if !ok {
		return
	}
	var addrs []string
	for _, i := range ingress {
		if iMap, ok := i.(map[string]any); ok {
			if ip, ok := iMap["ip"].(string); ok {
				addrs = append(addrs, ip)
			} else if host, ok := iMap["hostname"].(string); ok {
				addrs = append(addrs, host)
			}
		}
	}
	if len(addrs) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: columnKey, Value: strings.Join(addrs, ", ")})
	}
}

// populateServiceSelector extracts and formats the selector from a Service spec.
func populateServiceSelector(ti *model.Item, spec map[string]any) {
	selector, ok := spec["selector"].(map[string]any)
	if !ok {
		return
	}
	var parts []string
	for k, v := range selector {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	sort.Strings(parts)
	if len(parts) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Selector", Value: strings.Join(parts, ", ")})
	}
}

// populateIngressDetails extracts ingress class, rules, hosts, default backend,
// TLS hosts, URL, and load balancer addresses for an Ingress.
func populateIngressDetails(ti *model.Item, status, spec map[string]any) {
	if spec == nil {
		return
	}
	// Ingress class.
	if ic, ok := spec["ingressClassName"].(string); ok && ic != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ingress Class", Value: ic})
	}
	populateIngressRulesAndHosts(ti, spec)
	populateIngressDefaultBackend(ti, spec)
	tlsHostSet := populateIngressTLSHosts(ti, spec)
	populateIngressURL(ti, spec, tlsHostSet)
	populateLoadBalancerAddresses(ti, status, "Address")
}

// populateIngressRulesAndHosts extracts rule count and host names from an Ingress spec.
func populateIngressRulesAndHosts(ti *model.Item, spec map[string]any) {
	rules, ok := spec["rules"].([]any)
	if !ok {
		return
	}
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Rules", Value: fmt.Sprintf("%d", len(rules))})
	var hosts []string
	for _, r := range rules {
		if rMap, ok := r.(map[string]any); ok {
			if host, ok := rMap["host"].(string); ok {
				hosts = append(hosts, host)
			}
		}
	}
	if len(hosts) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Hosts", Value: strings.Join(hosts, ", ")})
	}
}

// populateIngressDefaultBackend extracts the default backend from an Ingress spec.
func populateIngressDefaultBackend(ti *model.Item, spec map[string]any) {
	defBackend, ok := spec["defaultBackend"].(map[string]any)
	if !ok {
		return
	}
	svc, ok := defBackend["service"].(map[string]any)
	if !ok {
		return
	}
	svcName, _ := svc["name"].(string)
	if port, ok := svc["port"].(map[string]any); ok {
		if num, ok := port["number"].(float64); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Default Backend", Value: fmt.Sprintf("%s:%d", svcName, int64(num))})
		} else if name, ok := port["name"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Default Backend", Value: fmt.Sprintf("%s:%s", svcName, name)})
		}
	} else if svcName != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Default Backend", Value: svcName})
	}
}

// populateIngressTLSHosts extracts TLS hosts from an Ingress spec and returns
// the set of TLS-enabled hosts for URL scheme detection.
func populateIngressTLSHosts(ti *model.Item, spec map[string]any) map[string]bool {
	tls, ok := spec["tls"].([]any)
	if !ok || len(tls) == 0 {
		return nil
	}
	tlsHostSet := make(map[string]bool)
	var tlsHosts []string
	for _, t := range tls {
		if tMap, ok := t.(map[string]any); ok {
			if hosts, ok := tMap["hosts"].([]any); ok {
				for _, h := range hosts {
					if s, ok := h.(string); ok {
						tlsHosts = append(tlsHosts, s)
						tlsHostSet[s] = true
					}
				}
			}
		}
	}
	if len(tlsHosts) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "TLS Hosts", Value: strings.Join(tlsHosts, ", ")})
	}
	return tlsHostSet
}

// populateIngressURL builds a URL from the first rule's host and path for "Open in Browser".
func populateIngressURL(ti *model.Item, spec map[string]any, tlsHostSet map[string]bool) {
	rules, ok := spec["rules"].([]any)
	if !ok || len(rules) == 0 {
		return
	}
	firstRule, ok := rules[0].(map[string]any)
	if !ok {
		return
	}
	host, ok := firstRule["host"].(string)
	if !ok || host == "" {
		return
	}
	scheme := "http"
	if tlsHostSet[host] {
		scheme = "https"
	}
	path := ""
	if httpBlock, ok := firstRule["http"].(map[string]any); ok {
		if paths, ok := httpBlock["paths"].([]any); ok && len(paths) > 0 {
			if firstPath, ok := paths[0].(map[string]any); ok {
				if p, ok := firstPath["path"].(string); ok && p != "" && p != "/" {
					path = p
				}
			}
		}
	}
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "__ingress_url", Value: scheme + "://" + host + path})
}

// populateConfigMapDetails extracts data keys and values from a ConfigMap.
func populateConfigMapDetails(ti *model.Item, obj map[string]any) {
	data, ok := obj["data"].(map[string]any)
	if !ok {
		return
	}
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// Store ConfigMap data values with "data:" prefix for preview display.
	for _, k := range keys {
		if v, ok := data[k].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "data:" + k, Value: v})
		}
	}
}

// populateSecretDetails extracts and decodes secret data from a Secret.
func populateSecretDetails(ti *model.Item, obj map[string]any) {
	if data, ok := obj["data"].(map[string]any); ok {
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		// Store decoded secret values with "secret:" prefix for conditional display.
		for _, k := range keys {
			if encoded, ok := data[k].(string); ok {
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				if err == nil {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "secret:" + k, Value: string(decoded)})
				}
			}
		}
	}
	if sType, ok := obj["type"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Type", Value: sType})
	}
}

// populateNodeDetails extracts roles, addresses, allocatable resources,
// node info, and taints for a Node.
func populateNodeDetails(ti *model.Item, obj map[string]any, status, spec map[string]any) {
	populateNodeRoles(ti, obj)
	populateNodeStatus(ti, status)
	populateNodeTaints(ti, spec)
}

// populateNodeRoles extracts node roles from labels.
func populateNodeRoles(ti *model.Item, obj map[string]any) {
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		return
	}
	labels, ok := metadata["labels"].(map[string]any)
	if !ok {
		return
	}
	var roles []string
	for k := range labels {
		if after, ok0 := strings.CutPrefix(k, "node-role.kubernetes.io/"); ok0 {
			role := after
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	if len(roles) > 0 {
		sort.Strings(roles)
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Role", Value: strings.Join(roles, ",")})
	}
}

// populateNodeStatus extracts addresses, allocatable resources, and node info
// from the Node status.
func populateNodeStatus(ti *model.Item, status map[string]any) {
	if status == nil {
		return
	}
	if addrs, ok := status["addresses"].([]any); ok {
		for _, a := range addrs {
			if aMap, ok := a.(map[string]any); ok {
				addrType, _ := aMap["type"].(string)
				addr, _ := aMap["address"].(string)
				if addrType != "" && addr != "" {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: addrType, Value: addr})
				}
			}
		}
	}
	// Add allocatable CPU/Memory as hidden data columns for metrics enrichment.
	if alloc, ok := status["allocatable"].(map[string]any); ok {
		if cpu, ok := alloc["cpu"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "CPU Alloc", Value: cpu})
		}
		if mem, ok := alloc["memory"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Mem Alloc", Value: mem})
		}
	}
	if nodeInfo, ok := status["nodeInfo"].(map[string]any); ok {
		if v, ok := nodeInfo["kubeletVersion"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Version", Value: v})
		}
		if v, ok := nodeInfo["osImage"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "OS", Value: v})
		}
		if v, ok := nodeInfo["containerRuntimeVersion"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Runtime", Value: v})
		}
	}
}

// populateNodeTaints extracts taints from the Node spec.
func populateNodeTaints(ti *model.Item, spec map[string]any) {
	if spec == nil {
		return
	}
	taints, ok := spec["taints"].([]any)
	if !ok || len(taints) == 0 {
		return
	}
	var taintStrs []string
	for _, t := range taints {
		if tMap, ok := t.(map[string]any); ok {
			key, _ := tMap["key"].(string)
			value, _ := tMap["value"].(string)
			effect, _ := tMap["effect"].(string)
			taint := key
			if value != "" {
				taint += "=" + value
			}
			taint += ":" + effect
			taintStrs = append(taintStrs, taint)
		}
	}
	if len(taintStrs) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Taints", Value: strings.Join(taintStrs, ", ")})
	}
}

// populatePVCDetails extracts phase, capacity, request, volume name,
// access modes, storage class, and volume mode for a PersistentVolumeClaim.
func populatePVCDetails(ti *model.Item, status, spec map[string]any) {
	// Phase/status -- set ti.Status only; the built-in Status column displays it.
	if status != nil {
		if phase, ok := status["phase"].(string); ok {
			ti.Status = phase
		}
		// Actual capacity from status (may differ from requested).
		if cap, ok := status["capacity"].(map[string]any); ok {
			if storage, ok := cap["storage"].(string); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Capacity", Value: storage})
			}
		}
	}
	if spec == nil {
		return
	}
	// Requested storage (show if no status capacity yet).
	if res, ok := spec["resources"].(map[string]any); ok {
		if req, ok := res["requests"].(map[string]any); ok {
			if storage, ok := req["storage"].(string); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Request", Value: storage})
			}
		}
	}
	// Volume name.
	if vol, ok := spec["volumeName"].(string); ok && vol != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Volume", Value: vol})
	}
	if am, ok := spec["accessModes"].([]any); ok {
		var modes []string
		for _, m := range am {
			if s, ok := m.(string); ok {
				modes = append(modes, s)
			}
		}
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Access Modes", Value: strings.Join(modes, ", ")})
	}
	if sc, ok := spec["storageClassName"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Storage Class", Value: sc})
	}
	if vm, ok := spec["volumeMode"].(string); ok && vm != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Volume Mode", Value: vm})
	}
}

// populateCronJobDetails extracts schedule, suspend, and last/next schedule
// time for a CronJob. Columns are emitted in the order they should appear:
// Schedule (what), Last Schedule (when it last ran), Next (when it runs next),
// Suspend (operational state — least likely to need at a glance).
func populateCronJobDetails(ti *model.Item, status, spec map[string]any) {
	var (
		schedule string
		timeZone string
		suspend  bool
	)
	if spec != nil {
		if sched, ok := spec["schedule"].(string); ok {
			schedule = sched
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Schedule", Value: sched})
		}
		if tz, ok := spec["timeZone"].(string); ok {
			timeZone = tz
		}
		if s, ok := spec["suspend"].(bool); ok {
			suspend = s
		}
	}
	if status != nil {
		if lastSchedule, ok := status["lastScheduleTime"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Last Schedule", Value: lastSchedule})
		}
	}
	if !suspend && schedule != "" {
		if next, ok := nextCronFire(schedule, timeZone, time.Now()); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Next", Value: formatAge(time.Until(next))})
		}
	}
	if spec != nil {
		if _, ok := spec["suspend"].(bool); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Suspend", Value: fmt.Sprintf("%v", suspend)})
		}
	}
}

// populateJobDetails extracts succeeded/failed counts, completions target, and
// suspend state for a Job. Columns ordered to match user attention: progress
// counts first, then operational state.
func populateJobDetails(ti *model.Item, status, spec map[string]any) {
	if spec != nil {
		if completions, ok := spec["completions"].(float64); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Completions", Value: fmt.Sprintf("%d", int64(completions))})
		}
	}
	if status != nil {
		if succeeded, ok := status["succeeded"].(float64); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Succeeded", Value: fmt.Sprintf("%d", int64(succeeded))})
		}
		if failed, ok := status["failed"].(float64); ok && failed > 0 {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Failed", Value: fmt.Sprintf("%d", int64(failed))})
		}
	}
	if spec != nil {
		if suspend, ok := spec["suspend"].(bool); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Suspend", Value: fmt.Sprintf("%v", suspend)})
		}
	}
}

// populateHPADetails extracts replica counts, targets, metrics, and conditions
// for a HorizontalPodAutoscaler.
func populateHPADetails(ti *model.Item, status, spec map[string]any) {
	populateHPAReady(ti, status, spec)
	populateHPASpecColumns(ti, spec)
	populateHPAStatusColumns(ti, status)
}

// populateHPAReady sets the Ready field to show current/desired replicas for an HPA.
func populateHPAReady(ti *model.Item, status, spec map[string]any) {
	if status == nil {
		return
	}
	var currentR, desiredR int64
	if cr, ok := status["currentReplicas"].(float64); ok {
		currentR = int64(cr)
	}
	if dr, ok := status["desiredReplicas"].(float64); ok {
		desiredR = int64(dr)
	}
	// Show min/max from spec for context.
	var minR, maxR int64
	if spec != nil {
		if mr, ok := spec["minReplicas"].(float64); ok {
			minR = int64(mr)
		}
		if mr, ok := spec["maxReplicas"].(float64); ok {
			maxR = int64(mr)
		}
	}
	ti.Ready = fmt.Sprintf("%d/%d (%d-%d)", currentR, desiredR, minR, maxR)
}

// populateHPASpecColumns extracts target reference, min/max replicas, and metric
// targets from an HPA spec.
func populateHPASpecColumns(ti *model.Item, spec map[string]any) {
	if spec == nil {
		return
	}
	// Target reference.
	if scaleTargetRef, ok := spec["scaleTargetRef"].(map[string]any); ok {
		refKind, _ := scaleTargetRef["kind"].(string)
		refName, _ := scaleTargetRef["name"].(string)
		if refKind != "" && refName != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Target", Value: refKind + "/" + refName})
		}
	}
	if minR, ok := spec["minReplicas"].(float64); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Min Replicas", Value: fmt.Sprintf("%d", int64(minR))})
	}
	if maxR, ok := spec["maxReplicas"].(float64); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Max Replicas", Value: fmt.Sprintf("%d", int64(maxR))})
	}
	// Metrics from spec (target values).
	if metrics, ok := spec["metrics"].([]any); ok {
		populateHPASpecMetrics(ti, metrics)
	}
}

// populateHPASpecMetrics extracts metric target values from the HPA spec metrics array.
func populateHPASpecMetrics(ti *model.Item, metrics []any) {
	for _, m := range metrics {
		mMap, ok := m.(map[string]any)
		if !ok {
			continue
		}
		mType, _ := mMap["type"].(string)
		switch mType {
		case "Resource":
			populateHPAResourceMetric(ti, mMap, "Target")
		case "Pods":
			populateHPAPodsMetric(ti, mMap, "target", "Target")
		case "Object":
			populateHPAObjectMetric(ti, mMap)
		}
	}
}

// populateHPAResourceMetric extracts a Resource metric (CPU/memory utilization or average value).
func populateHPAResourceMetric(ti *model.Item, mMap map[string]any, prefix string) {
	res, ok := mMap["resource"].(map[string]any)
	if !ok {
		return
	}
	resName, _ := res["name"].(string)
	target, ok := res["target"].(map[string]any)
	if !ok {
		return
	}
	targetType, _ := target["type"].(string)
	switch targetType {
	case "Utilization":
		if avg, ok := target["averageUtilization"].(float64); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{
				Key:   fmt.Sprintf("%s %s", prefix, strings.ToUpper(resName[:1])+resName[1:]),
				Value: fmt.Sprintf("%d%%", int64(avg)),
			})
		}
	case "AverageValue":
		if avg, ok := target["averageValue"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{
				Key:   fmt.Sprintf("%s %s", prefix, strings.ToUpper(resName[:1])+resName[1:]),
				Value: avg,
			})
		}
	}
}

// populateHPAPodsMetric extracts a Pods metric value.
func populateHPAPodsMetric(ti *model.Item, mMap map[string]any, dataKey, prefix string) {
	pods, ok := mMap["pods"].(map[string]any)
	if !ok {
		return
	}
	metricName := ""
	if mn, ok := pods["metric"].(map[string]any); ok {
		metricName, _ = mn["name"].(string)
	}
	data, ok := pods[dataKey].(map[string]any)
	if !ok {
		return
	}
	if avg, ok := data["averageValue"].(string); ok && metricName != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{
			Key:   fmt.Sprintf("%s %s", prefix, metricName),
			Value: avg,
		})
	}
}

// populateHPAObjectMetric extracts an Object metric value.
func populateHPAObjectMetric(ti *model.Item, mMap map[string]any) {
	object, ok := mMap["object"].(map[string]any)
	if !ok {
		return
	}
	metricName := ""
	if mn, ok := object["metric"].(map[string]any); ok {
		metricName, _ = mn["name"].(string)
	}
	target, ok := object["target"].(map[string]any)
	if !ok {
		return
	}
	if val, ok := target["value"].(string); ok && metricName != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{
			Key:   fmt.Sprintf("Target %s", metricName),
			Value: val,
		})
	}
}

// populateHPAStatusColumns extracts current replicas, desired replicas,
// current metrics, and conditions from the HPA status.
func populateHPAStatusColumns(ti *model.Item, status map[string]any) {
	if status == nil {
		return
	}
	if current, ok := status["currentReplicas"].(float64); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Current Replicas", Value: fmt.Sprintf("%d", int64(current))})
	}
	if desired, ok := status["desiredReplicas"].(float64); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Desired Replicas", Value: fmt.Sprintf("%d", int64(desired))})
	}
	// Current metrics from status.
	if currentMetrics, ok := status["currentMetrics"].([]any); ok {
		populateHPACurrentMetrics(ti, currentMetrics)
	}
	// Conditions summary.
	if conditions, ok := status["conditions"].([]any); ok {
		populateHPAConditions(ti, conditions)
	}
}

// populateHPACurrentMetrics extracts current metric values from the HPA status.
func populateHPACurrentMetrics(ti *model.Item, currentMetrics []any) {
	for _, m := range currentMetrics {
		mMap, ok := m.(map[string]any)
		if !ok {
			continue
		}
		mType, _ := mMap["type"].(string)
		switch mType {
		case "Resource":
			populateHPACurrentResourceMetric(ti, mMap)
		case "Pods":
			populateHPACurrentPodsMetric(ti, mMap)
		}
	}
}

// populateHPACurrentResourceMetric extracts the current value for a Resource metric.
func populateHPACurrentResourceMetric(ti *model.Item, mMap map[string]any) {
	res, ok := mMap["resource"].(map[string]any)
	if !ok {
		return
	}
	resName, _ := res["name"].(string)
	current, ok := res["current"].(map[string]any)
	if !ok {
		return
	}
	if avg, ok := current["averageUtilization"].(float64); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{
			Key:   fmt.Sprintf("Current %s", strings.ToUpper(resName[:1])+resName[1:]),
			Value: fmt.Sprintf("%d%%", int64(avg)),
		})
	} else if avgVal, ok := current["averageValue"].(string); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{
			Key:   fmt.Sprintf("Current %s", strings.ToUpper(resName[:1])+resName[1:]),
			Value: avgVal,
		})
	}
}

// populateHPACurrentPodsMetric extracts the current value for a Pods metric.
func populateHPACurrentPodsMetric(ti *model.Item, mMap map[string]any) {
	pods, ok := mMap["pods"].(map[string]any)
	if !ok {
		return
	}
	metricName := ""
	if mn, ok := pods["metric"].(map[string]any); ok {
		metricName, _ = mn["name"].(string)
	}
	current, ok := pods["current"].(map[string]any)
	if !ok {
		return
	}
	if avg, ok := current["averageValue"].(string); ok && metricName != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{
			Key:   fmt.Sprintf("Current %s", metricName),
			Value: avg,
		})
	}
}

// populateHPAConditions extracts the ScalingLimited condition from HPA status.
func populateHPAConditions(ti *model.Item, conditions []any) {
	for _, c := range conditions {
		cMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		cType, _ := cMap["type"].(string)
		cStatus, _ := cMap["status"].(string)
		if cType == "ScalingLimited" && cStatus == "True" {
			msg, _ := cMap["message"].(string)
			if msg != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Scaling Limited", Value: msg})
			}
		}
	}
}
