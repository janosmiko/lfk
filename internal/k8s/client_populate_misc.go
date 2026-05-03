package k8s

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

func populateIngressClass(ti *model.Item, obj map[string]any) {
	metadata, _ := obj["metadata"].(map[string]any)
	annotations, _ := metadata["annotations"].(map[string]any)
	if val, ok := annotations["ingressclass.kubernetes.io/is-default-class"].(string); ok && val == "true" {
		ti.Name += " (default)"
		ti.Status = "default"
	}
}

func populateStorageClass(ti *model.Item, obj map[string]any) {
	metadata, _ := obj["metadata"].(map[string]any)
	annotations, _ := metadata["annotations"].(map[string]any)
	if val, ok := annotations["storageclass.kubernetes.io/is-default-class"].(string); ok && val == "true" {
		ti.Name += " (default)"
		ti.Status = "default"
	}
	if provisioner, ok := obj["provisioner"].(string); ok && provisioner != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Provisioner", Value: provisioner})
	}
	if reclaimPolicy, ok := obj["reclaimPolicy"].(string); ok && reclaimPolicy != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reclaim Policy", Value: reclaimPolicy})
	}
	if vbm, ok := obj["volumeBindingMode"].(string); ok && vbm != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Binding Mode", Value: vbm})
	}
	if ae, ok := obj["allowVolumeExpansion"].(bool); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Allow Expansion", Value: fmt.Sprintf("%v", ae)})
	}
}

func populateServiceAccount(ti *model.Item, obj map[string]any) {
	if secrets, ok := obj["secrets"].([]any); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Secrets", Value: fmt.Sprintf("%d", len(secrets))})
	}
	if automount, ok := obj["automountServiceAccountToken"].(bool); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Automount Token", Value: fmt.Sprintf("%v", automount)})
	}
	if ips, ok := obj["imagePullSecrets"].([]any); ok && len(ips) > 0 {
		var names []string
		for _, s := range ips {
			if sMap, ok := s.(map[string]any); ok {
				if name, ok := sMap["name"].(string); ok {
					names = append(names, name)
				}
			}
		}
		if len(names) > 0 {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Image Pull Secrets", Value: strings.Join(names, ", ")})
		}
	}
}

func populateEvent(ti *model.Item, obj map[string]any) {
	if eventType, ok := obj["type"].(string); ok {
		ti.Status = eventType
	}

	firstTime := parseEventTimestamp(obj, "firstTimestamp")
	lastTime := parseEventTimestamp(obj, "lastTimestamp")
	if firstTime.IsZero() {
		firstTime = parseEventTimestamp(obj, "eventTime")
	}
	if lastTime.IsZero() {
		lastTime = parseEventTimestamp(obj, "eventTime")
	}
	if firstTime.IsZero() && !lastTime.IsZero() {
		firstTime = lastTime
	}
	if lastTime.IsZero() && !firstTime.IsZero() {
		lastTime = firstTime
	}

	if !firstTime.IsZero() {
		ti.CreatedAt = firstTime
		ti.Age = formatAge(time.Since(firstTime))
	}
	if !lastTime.IsZero() {
		ti.LastSeen = lastTime
	}

	if involvedObj, ok := obj["involvedObject"].(map[string]any); ok {
		objKind, _ := involvedObj["kind"].(string)
		objName, _ := involvedObj["name"].(string)
		if objKind != "" && objName != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: EventColObject, Value: objKind + "/" + objName})
		}
	}
	if reason, ok := obj["reason"].(string); ok && reason != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: EventColReason, Value: reason})
	}
	if message, ok := obj["message"].(string); ok && message != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: EventColMessage, Value: message})
	}
	eventCount := int64(1)
	if count, ok := obj["count"].(int64); ok && count > 0 {
		eventCount = count
	} else if countF, ok := obj["count"].(float64); ok && countF > 0 {
		eventCount = int64(countF)
	}
	ti.Columns = append(ti.Columns, model.KeyValue{Key: EventColCount, Value: fmt.Sprintf("%d", eventCount)})
	if source, ok := obj["source"].(map[string]any); ok {
		if component, ok := source["component"].(string); ok && component != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: EventColSource, Value: component})
		}
	}
	if !lastTime.IsZero() {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: EventColLastSeen, Value: formatAge(time.Since(lastTime))})
	}
}

func populatePersistentVolume(ti *model.Item, status, spec map[string]any) {
	if spec != nil {
		if cap, ok := spec["capacity"].(map[string]any); ok {
			if storage, ok := cap["storage"].(string); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Capacity", Value: storage})
			}
		}
		if am, ok := spec["accessModes"].([]any); ok {
			var modes []string
			for _, m := range am {
				if s, ok := m.(string); ok {
					modes = append(modes, s)
				}
			}
			if len(modes) > 0 {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Access Modes", Value: strings.Join(modes, ", ")})
			}
		}
		if rp, ok := spec["persistentVolumeReclaimPolicy"].(string); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reclaim Policy", Value: rp})
		}
		if sc, ok := spec["storageClassName"].(string); ok && sc != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Storage Class", Value: sc})
		}
		if vm, ok := spec["volumeMode"].(string); ok && vm != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Volume Mode", Value: vm})
		}
		if claimRef, ok := spec["claimRef"].(map[string]any); ok {
			claimNS, _ := claimRef["namespace"].(string)
			claimName, _ := claimRef["name"].(string)
			if claimName != "" {
				claim := claimName
				if claimNS != "" {
					claim = claimNS + "/" + claimName
				}
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Claim", Value: claim})
			}
		}
	}
	if status != nil {
		if phase, ok := status["phase"].(string); ok {
			ti.Status = phase
		}
		if reason, ok := status["reason"].(string); ok && reason != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reason", Value: reason})
		}
	}
}

func populateResourceQuota(ti *model.Item, status, spec map[string]any) {
	if status != nil {
		hard, _ := status["hard"].(map[string]any)
		used, _ := status["used"].(map[string]any)
		if hard != nil {
			quotaKeys := make([]string, 0, len(hard))
			for k := range hard {
				quotaKeys = append(quotaKeys, k)
			}
			sort.Strings(quotaKeys)
			for _, k := range quotaKeys {
				hardVal := fmt.Sprintf("%v", hard[k])
				usedVal := "0"
				if used != nil {
					if u, ok := used[k]; ok {
						usedVal = fmt.Sprintf("%v", u)
					}
				}
				ti.Columns = append(ti.Columns, model.KeyValue{
					Key:   k,
					Value: fmt.Sprintf("%s / %s", usedVal, hardVal),
				})
			}
		}
	} else if spec != nil {
		if hard, ok := spec["hard"].(map[string]any); ok {
			quotaKeys := make([]string, 0, len(hard))
			for k := range hard {
				quotaKeys = append(quotaKeys, k)
			}
			sort.Strings(quotaKeys)
			for _, k := range quotaKeys {
				ti.Columns = append(ti.Columns, model.KeyValue{
					Key:   k,
					Value: fmt.Sprintf("%v (hard)", hard[k]),
				})
			}
		}
	}
}

func populateLimitRange(ti *model.Item, spec map[string]any) {
	if spec == nil {
		return
	}
	limits, ok := spec["limits"].([]any)
	if !ok {
		return
	}
	for _, l := range limits {
		lMap, ok := l.(map[string]any)
		if !ok {
			continue
		}
		lType, _ := lMap["type"].(string)
		prefix := lType
		if prefix == "" {
			prefix = "Unknown"
		}
		if def, ok := lMap["default"].(map[string]any); ok {
			for resource, val := range def {
				ti.Columns = append(ti.Columns, model.KeyValue{
					Key:   fmt.Sprintf("%s Default %s", prefix, resource),
					Value: fmt.Sprintf("%v", val),
				})
			}
		}
		if defReq, ok := lMap["defaultRequest"].(map[string]any); ok {
			for resource, val := range defReq {
				ti.Columns = append(ti.Columns, model.KeyValue{
					Key:   fmt.Sprintf("%s Default Req %s", prefix, resource),
					Value: fmt.Sprintf("%v", val),
				})
			}
		}
		if max, ok := lMap["max"].(map[string]any); ok {
			for resource, val := range max {
				ti.Columns = append(ti.Columns, model.KeyValue{
					Key:   fmt.Sprintf("%s Max %s", prefix, resource),
					Value: fmt.Sprintf("%v", val),
				})
			}
		}
		if min, ok := lMap["min"].(map[string]any); ok {
			for resource, val := range min {
				ti.Columns = append(ti.Columns, model.KeyValue{
					Key:   fmt.Sprintf("%s Min %s", prefix, resource),
					Value: fmt.Sprintf("%v", val),
				})
			}
		}
	}
}

func populatePodDisruptionBudget(ti *model.Item, status, spec map[string]any) {
	if spec != nil {
		if minAvail, ok := spec["minAvailable"]; ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Min Available", Value: fmt.Sprintf("%v", minAvail)})
		}
		if maxUnavail, ok := spec["maxUnavailable"]; ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Max Unavailable", Value: fmt.Sprintf("%v", maxUnavail)})
		}
		if selector, ok := spec["selector"].(map[string]any); ok {
			if matchLabels, ok := selector["matchLabels"].(map[string]any); ok {
				parts := make([]string, 0, len(matchLabels))
				for k, v := range matchLabels {
					parts = append(parts, fmt.Sprintf("%s=%v", k, v))
				}
				if len(parts) > 0 {
					sort.Strings(parts)
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Selector", Value: strings.Join(parts, ", ")})
				}
			}
		}
	}
	if status != nil {
		if current, ok := status["currentHealthy"].(float64); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Current Healthy", Value: fmt.Sprintf("%d", int64(current))})
		}
		if desired, ok := status["desiredHealthy"].(float64); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Desired Healthy", Value: fmt.Sprintf("%d", int64(desired))})
		}
		if allowed, ok := status["disruptionsAllowed"].(float64); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Disruptions Allowed", Value: fmt.Sprintf("%d", int64(allowed))})
		}
		if expected, ok := status["expectedPods"].(float64); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Expected Pods", Value: fmt.Sprintf("%d", int64(expected))})
		}
	}
}

func populateNetworkPolicy(ti *model.Item, spec map[string]any) {
	if spec == nil {
		return
	}
	if selector, ok := spec["podSelector"].(map[string]any); ok {
		if matchLabels, ok := selector["matchLabels"].(map[string]any); ok {
			var parts []string
			for k, v := range matchLabels {
				parts = append(parts, fmt.Sprintf("%s=%v", k, v))
			}
			if len(parts) > 0 {
				sort.Strings(parts)
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Pod Selector", Value: strings.Join(parts, ", ")})
			}
		} else {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Pod Selector", Value: "(all pods)"})
		}
	}
	if policyTypes, ok := spec["policyTypes"].([]any); ok {
		var types []string
		for _, pt := range policyTypes {
			if s, ok := pt.(string); ok {
				types = append(types, s)
			}
		}
		if len(types) > 0 {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Policy Types", Value: strings.Join(types, ", ")})
		}
	}
	if ingress, ok := spec["ingress"].([]any); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ingress Rules", Value: fmt.Sprintf("%d", len(ingress))})
	}
	if egress, ok := spec["egress"].([]any); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Egress Rules", Value: fmt.Sprintf("%d", len(egress))})
	}
}

// endpointEntry is the per-address payload extracted from a v1 Endpoints
// subset or a discovery.k8s.io/v1 EndpointSlice endpoint. Used to build
// the multi-line "Endpoints" preview block uniformly across both kinds.
type endpointEntry struct {
	addr       string
	targetKind string
	targetName string
	nodeName   string
	ready      bool
}

// formatEndpointLine renders a single endpoint as
// "addr → kind/name on node (NotReady)" for the per-endpoint multi-line
// "Endpoints" preview field. The (NotReady) suffix is only emitted when
// the endpoint is *not* ready — ready is the silent default so the eye
// is drawn to broken endpoints. Missing target ref / node degrade the
// line gracefully (drop the segment).
func formatEndpointLine(addr, targetKind, targetName, nodeName string, ready bool) string {
	parts := []string{addr}
	if targetKind != "" && targetName != "" {
		parts = append(parts, "→", strings.ToLower(targetKind)+"/"+targetName)
	}
	if nodeName != "" {
		parts = append(parts, "on", nodeName)
	}
	line := strings.Join(parts, " ")
	if !ready {
		line += " (NotReady)"
	}
	return line
}

// joinEndpointLines turns a list of endpointEntry values into the
// newline-separated value of the "Endpoints" KeyValue. The renderer
// (extended in this PR to split on `\n` for the "Endpoints" key) emits
// one preview line per entry.
func joinEndpointLines(entries []endpointEntry) string {
	if len(entries) == 0 {
		return ""
	}
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		lines = append(lines, formatEndpointLine(e.addr, e.targetKind, e.targetName, e.nodeName, e.ready))
	}
	return strings.Join(lines, "\n")
}

// extractTargetRef pulls (kind, name) from a targetRef map. Returns
// empty strings when the ref is absent or malformed — formatEndpointLine
// degrades gracefully on missing values.
func extractTargetRef(m map[string]any) (kind, name string) {
	ref, ok := m["targetRef"].(map[string]any)
	if !ok {
		return "", ""
	}
	kind, _ = ref["kind"].(string)
	name, _ = ref["name"].(string)
	return kind, name
}

func populateEndpoints(ti *model.Item, obj map[string]any) {
	subsets, ok := obj["subsets"].([]any)
	if !ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Endpoints", Value: "<none>"})
		return
	}
	var entries []endpointEntry
	var portStrs []string
	for _, s := range subsets {
		subset, ok := s.(map[string]any)
		if !ok {
			continue
		}
		entries = append(entries, collectV1EndpointAddresses(subset, "addresses", true)...)
		entries = append(entries, collectV1EndpointAddresses(subset, "notReadyAddresses", false)...)
		if list, ok := subset["ports"].([]any); ok {
			for _, p := range list {
				if pmap, ok := p.(map[string]any); ok {
					portStrs = append(portStrs, formatEndpointPort(pmap))
				}
			}
		}
	}
	emitEndpointPreviewColumns(ti, entries, portStrs)
}

// collectV1EndpointAddresses pulls one of "addresses" or "notReadyAddresses"
// from a v1 Endpoints subset and returns endpointEntry values flagged with
// the supplied ready state.
func collectV1EndpointAddresses(subset map[string]any, key string, ready bool) []endpointEntry {
	list, ok := subset[key].([]any)
	if !ok {
		return nil
	}
	out := make([]endpointEntry, 0, len(list))
	for _, a := range list {
		amap, ok := a.(map[string]any)
		if !ok {
			continue
		}
		ip, _ := amap["ip"].(string)
		if ip == "" {
			continue
		}
		nodeName, _ := amap["nodeName"].(string)
		kind, name := extractTargetRef(amap)
		out = append(out, endpointEntry{
			addr:       ip,
			targetKind: kind,
			targetName: name,
			nodeName:   nodeName,
			ready:      ready,
		})
	}
	return out
}

// emitEndpointPreviewColumns appends the Ready / Not Ready / Endpoints /
// Ports preview columns from a fully-collected entry slice. Shared by
// both populateEndpoints and populateEndpointSlice so the two kinds
// stay visually identical even though they read different API shapes.
func emitEndpointPreviewColumns(ti *model.Item, entries []endpointEntry, portStrs []string) {
	var ready, notReady int
	for _, e := range entries {
		if e.ready {
			ready++
		} else {
			notReady++
		}
	}
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ready", Value: fmt.Sprintf("%d", ready)})
	if notReady > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Not Ready", Value: fmt.Sprintf("%d", notReady)})
	}
	if v := joinEndpointLines(entries); v != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Endpoints", Value: v})
	}
	if len(portStrs) > 0 {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ports", Value: strings.Join(portStrs, ", ")})
	}
}

func populateEndpointSlice(ti *model.Item, obj map[string]any) {
	if t, ok := obj["addressType"].(string); ok && t != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Type", Value: t})
	}
	endpoints, _ := obj["endpoints"].([]any)
	var entries []endpointEntry
	for _, e := range endpoints {
		ep, ok := e.(map[string]any)
		if !ok {
			continue
		}
		// Per discovery.k8s.io/v1: a missing or null conditions.ready is
		// "unknown" and consumers should interpret unknown as *ready*
		// (per the upstream API spec — older API versions didn't
		// populate the field, so absence means "not enough info to
		// declare not-ready"). Default to true and only flip to false
		// when ready is explicitly present and false. The previous
		// inverted default would have flagged every endpoint on an
		// older slice as (NotReady).
		isReady := true
		if cond, ok := ep["conditions"].(map[string]any); ok {
			if r, ok := cond["ready"].(bool); ok {
				isReady = r
			}
		}
		nodeName, _ := ep["nodeName"].(string)
		kind, name := extractTargetRef(ep)
		if as, ok := ep["addresses"].([]any); ok {
			for _, a := range as {
				s, ok := a.(string)
				if !ok || s == "" {
					continue
				}
				entries = append(entries, endpointEntry{
					addr:       s,
					targetKind: kind,
					targetName: name,
					nodeName:   nodeName,
					ready:      isReady,
				})
			}
		}
	}
	var portStrs []string
	if ports, ok := obj["ports"].([]any); ok {
		for _, p := range ports {
			if pmap, ok := p.(map[string]any); ok {
				portStrs = append(portStrs, formatEndpointPort(pmap))
			}
		}
	}
	emitEndpointPreviewColumns(ti, entries, portStrs)
}

func formatEndpointPort(p map[string]any) string {
	port, _ := p["port"].(float64)
	proto, _ := p["protocol"].(string)
	name, _ := p["name"].(string)
	if proto == "" {
		proto = "TCP"
	}
	if name != "" {
		return fmt.Sprintf("%s:%d/%s", name, int64(port), proto)
	}
	return fmt.Sprintf("%d/%s", int64(port), proto)
}
