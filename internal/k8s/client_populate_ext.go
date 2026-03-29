package k8s

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/model"
)

// populateResourceDetailsExt handles extended resource kinds not covered by the core
// populateResourceDetails switch: FluxCD, cert-manager, ArgoCD, Events, storage types,
// RBAC-related types, and generic CRD fallback.
func populateResourceDetailsExt(ti *model.Item, obj map[string]interface{}, kind string, status, spec map[string]interface{}) {
	switch kind {
	case "Kustomization", "GitRepository", "HelmRepository", "HelmChart", "OCIRepository", "Bucket",
		"Alert", "Provider", "Receiver", "ImageRepository", "ImagePolicy", "ImageUpdateAutomation":
		// FluxCD resources: extract conditions-based status.
		if spec, ok := obj["spec"].(map[string]interface{}); ok {
			if suspended, ok := spec["suspend"].(bool); ok && suspended {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Suspended", Value: "True"})
			}
		}
		if status != nil {
			// Extract Ready condition.
			if conditions, ok := status["conditions"].([]interface{}); ok {
				for _, c := range conditions {
					cond, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					condType, _ := cond["type"].(string)
					condStatus, _ := cond["status"].(string)
					condMessage, _ := cond["message"].(string)
					condReason, _ := cond["reason"].(string)
					if condType == "Ready" {
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ready", Value: condStatus})
						if condReason != "" {
							ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reason", Value: condReason})
						}
						if condMessage != "" && condStatus != "True" {
							ti.Columns = append(ti.Columns, model.KeyValue{Key: "Message", Value: condMessage})
						}
						if lastTransition, ok := cond["lastTransitionTime"].(string); ok && lastTransition != "" {
							if t, err := time.Parse(time.RFC3339, lastTransition); err == nil {
								ti.Columns = append(ti.Columns, model.KeyValue{Key: "Last Transition", Value: formatRelativeTime(t)})
							}
						}
						break
					}
				}
			}
			// Extract last applied revision.
			if rev, ok := status["lastAppliedRevision"].(string); ok && rev != "" {
				if len(rev) > 12 {
					rev = rev[:12]
				}
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Revision", Value: rev})
			} else if artifact, ok := status["artifact"].(map[string]interface{}); ok {
				if rev, ok := artifact["revision"].(string); ok && rev != "" {
					if len(rev) > 12 {
						rev = rev[:12]
					}
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Revision", Value: rev})
				}
			}
		}

	case "Certificate", "CertificateRequest", "Issuer", "ClusterIssuer", "Order", "Challenge":
		// cert-manager resources: extract conditions-based status and certificate-specific fields.
		if status != nil {
			if conditions, ok := status["conditions"].([]interface{}); ok {
				for _, c := range conditions {
					cond, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					condType, _ := cond["type"].(string)
					condStatus, _ := cond["status"].(string)
					condMessage, _ := cond["message"].(string)
					condReason, _ := cond["reason"].(string)
					if condType == "Ready" {
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ready", Value: condStatus})
						if condReason != "" {
							ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reason", Value: condReason})
						}
						if condMessage != "" && condStatus != "True" {
							ti.Columns = append(ti.Columns, model.KeyValue{Key: "Message", Value: condMessage})
						}
						if lastTransition, ok := cond["lastTransitionTime"].(string); ok && lastTransition != "" {
							if t, err := time.Parse(time.RFC3339, lastTransition); err == nil {
								ti.Columns = append(ti.Columns, model.KeyValue{Key: "Last Transition", Value: formatRelativeTime(t)})
							}
						}
						break
					}
				}
			}
			if notAfter, ok := status["notAfter"].(string); ok && notAfter != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Expires", Value: notAfter})
			}
			if renewalTime, ok := status["renewalTime"].(string); ok && renewalTime != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Renewal", Value: renewalTime})
			}
		}
		if spec != nil {
			if secretName, ok := spec["secretName"].(string); ok && secretName != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Secret", Value: secretName})
			}
		}

	case "Application", "ApplicationSet":
		populateArgoCDApplication(ti, obj, status, spec, kind)

	case "Event":
		populateEvent(ti, obj)

	case "IngressClass":
		metadata, _ := obj["metadata"].(map[string]interface{})
		annotations, _ := metadata["annotations"].(map[string]interface{})
		if val, ok := annotations["ingressclass.kubernetes.io/is-default-class"].(string); ok && val == "true" {
			ti.Name += " (default)"
			ti.Status = "default"
		}

	case "StorageClass":
		metadata, _ := obj["metadata"].(map[string]interface{})
		annotations, _ := metadata["annotations"].(map[string]interface{})
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

	case "PersistentVolume":
		populatePersistentVolume(ti, status, spec)

	case "ResourceQuota":
		populateResourceQuota(ti, status, spec)

	case "LimitRange":
		populateLimitRange(ti, spec)

	case "PodDisruptionBudget":
		populatePodDisruptionBudget(ti, status, spec)

	case "NetworkPolicy":
		populateNetworkPolicy(ti, spec)

	case "ServiceAccount":
		// Secrets count.
		if secrets, ok := obj["secrets"].([]interface{}); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Secrets", Value: fmt.Sprintf("%d", len(secrets))})
		}
		// Automount token.
		if automount, ok := obj["automountServiceAccountToken"].(bool); ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Automount Token", Value: fmt.Sprintf("%v", automount)})
		}
		// Image pull secrets.
		if ips, ok := obj["imagePullSecrets"].([]interface{}); ok && len(ips) > 0 {
			var names []string
			for _, s := range ips {
				if sMap, ok := s.(map[string]interface{}); ok {
					if name, ok := sMap["name"].(string); ok {
						names = append(names, name)
					}
				}
			}
			if len(names) > 0 {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Image Pull Secrets", Value: strings.Join(names, ", ")})
			}
		}

	case "PriorityClass":
		if val, ok := spec["globalDefault"].(bool); ok && val {
			ti.Name += " (default)"
			ti.Status = "default"
		}

	default:
		// For unknown/CRD resources, extract top-level status fields.
		if status != nil {
			for _, key := range []string{"phase", "state", "health", "sync", "message", "reason"} {
				if v, ok := status[key]; ok {
					label := strings.ToUpper(key[:1]) + key[1:]
					switch val := v.(type) {
					case map[string]interface{}:
						for subKey, subVal := range val {
							subLabel := label + " " + strings.ToUpper(subKey[:1]) + subKey[1:]
							ti.Columns = append(ti.Columns, model.KeyValue{Key: subLabel, Value: fmt.Sprintf("%v", subVal)})
						}
					default:
						ti.Columns = append(ti.Columns, model.KeyValue{Key: label, Value: fmt.Sprintf("%v", val)})
					}
				}
			}

			if conditions, ok := status["conditions"].([]interface{}); ok && len(conditions) > 0 {
				extractGenericConditions(ti, conditions)
			}
		}
	}
}

func populateArgoCDApplication(ti *model.Item, _ map[string]interface{}, status, spec map[string]interface{}, kind string) {
	if status != nil {
		// Health and Sync details are shown in the DETAILS pane only;
		// the STATUS column already combines health/sync status.
		if health, ok := status["health"].(map[string]interface{}); ok {
			if msg, ok := health["message"].(string); ok && msg != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Health Message", Value: msg})
			}
		}
		if sync, ok := status["sync"].(map[string]interface{}); ok {
			if rev, ok := sync["revision"].(string); ok && rev != "" {
				if len(rev) > 8 {
					rev = rev[:8]
				}
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Revision", Value: rev})
			}
		}
		if opState, ok := status["operationState"].(map[string]interface{}); ok {
			if phase, ok := opState["phase"].(string); ok && phase != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Last Sync", Value: phase})
			}
			if msg, ok := opState["message"].(string); ok && msg != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Sync Message", Value: msg})
			}
			if finishedAt, ok := opState["finishedAt"].(string); ok && finishedAt != "" {
				if t, err := time.Parse(time.RFC3339, finishedAt); err == nil {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Synced At", Value: formatRelativeTime(t)})
				}
			}
			if syncResult, ok := opState["syncResult"].(map[string]interface{}); ok {
				if resources, ok := syncResult["resources"].([]interface{}); ok {
					var errs []string
					for _, r := range resources {
						rMap, ok := r.(map[string]interface{})
						if !ok {
							continue
						}
						rStatus, _ := rMap["status"].(string)
						if rStatus != "Synced" && rStatus != "" {
							kind, _ := rMap["kind"].(string)
							name, _ := rMap["name"].(string)
							msg, _ := rMap["message"].(string)
							if msg != "" {
								errs = append(errs, fmt.Sprintf("%s/%s: %s", kind, name, msg))
							}
						}
					}
					if len(errs) > 0 {
						ti.Columns = append(ti.Columns, model.KeyValue{Key: "Sync Errors", Value: strings.Join(errs, "; ")})
					}
				}
			}
		}
		if summary, ok := status["summary"].(map[string]interface{}); ok {
			if imgs, ok := summary["images"].([]interface{}); ok && len(imgs) > 0 {
				var imageStrs []string
				for _, img := range imgs {
					if s, ok := img.(string); ok {
						imageStrs = append(imageStrs, s)
					}
				}
				if len(imageStrs) > 0 {
					ti.Columns = append(ti.Columns, model.KeyValue{Key: "Images", Value: strings.Join(imageStrs, ", ")})
				}
			}
		}
	}
	// Extract conditions (e.g., ComparisonError, InvalidSpecError, SyncError).
	// A short "Condition" column shows the type names in the table view.
	// Full messages are stored as "condition:<type>" detail-only keys.
	if conditions, ok := status["conditions"].([]interface{}); ok {
		var condTypes []string
		for _, c := range conditions {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _ := cond["type"].(string)
			condMsg, _ := cond["message"].(string)
			if condType == "" {
				continue
			}
			condTypes = append(condTypes, condType)
			// Full message stored with "condition:" prefix so it's excluded
			// from the table (prefix-blocked) but shown in the DETAILS pane.
			value := condMsg
			if value == "" {
				value = "(no message)"
			}
			if lastTransition, ok := cond["lastTransitionTime"].(string); ok && lastTransition != "" {
				if t, err := time.Parse(time.RFC3339, lastTransition); err == nil {
					value += " (" + formatRelativeTime(t) + ")"
				}
			}
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "condition:" + condType, Value: value})
		}
		if len(condTypes) > 0 {
			val := strings.Join(condTypes, ", ")
			if len(val) > 15 {
				val = val[:14] + "~"
			}
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Condition", Value: val})
		}
	}
	if spec != nil {
		// AutoSync column: only for Application, not ApplicationSet.
		if kind == "Application" {
			autoSyncVal := "Off"
			if syncPolicy, ok := spec["syncPolicy"].(map[string]interface{}); ok {
				if automated, ok := syncPolicy["automated"].(map[string]interface{}); ok && automated != nil {
					autoSyncVal = "On"
					if sh, ok := automated["selfHeal"].(bool); ok && sh {
						autoSyncVal += "/SH"
					}
					if pr, ok := automated["prune"].(bool); ok && pr {
						autoSyncVal += "/P"
					}
				}
			}
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "AutoSync", Value: autoSyncVal})
		}

		if dest, ok := spec["destination"].(map[string]interface{}); ok {
			if ns, ok := dest["namespace"].(string); ok && ns != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Dest NS", Value: ns})
			}
			if server, ok := dest["server"].(string); ok && server != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Dest Server", Value: server})
			}
		}
		if source, ok := spec["source"].(map[string]interface{}); ok {
			if repo, ok := source["repoURL"].(string); ok && repo != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Repo", Value: repo})
			}
			if path, ok := source["path"].(string); ok && path != "" {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Path", Value: path})
			}
		}
	}
}

func populateEvent(ti *model.Item, obj map[string]interface{}) {
	if eventType, ok := obj["type"].(string); ok {
		ti.Status = eventType
	}
	eventTime := parseEventTimestamp(obj, "lastTimestamp")
	if eventTime.IsZero() {
		eventTime = parseEventTimestamp(obj, "eventTime")
	}
	if !eventTime.IsZero() {
		ti.CreatedAt = eventTime
		ti.Age = formatAge(time.Since(eventTime))
	}
	if involvedObj, ok := obj["involvedObject"].(map[string]interface{}); ok {
		objKind, _ := involvedObj["kind"].(string)
		objName, _ := involvedObj["name"].(string)
		if objKind != "" && objName != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Object", Value: objKind + "/" + objName})
		}
	}
	if reason, ok := obj["reason"].(string); ok && reason != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reason", Value: reason})
	}
	if message, ok := obj["message"].(string); ok && message != "" {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Message", Value: message})
	}
	eventCount := int64(1)
	if count, ok := obj["count"].(int64); ok && count > 0 {
		eventCount = count
	} else if countF, ok := obj["count"].(float64); ok && countF > 0 {
		eventCount = int64(countF)
	}
	ti.Columns = append(ti.Columns, model.KeyValue{Key: "Count", Value: fmt.Sprintf("%d", eventCount)})
	if source, ok := obj["source"].(map[string]interface{}); ok {
		if component, ok := source["component"].(string); ok && component != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Source", Value: component})
		}
	}
}

func populatePersistentVolume(ti *model.Item, status, spec map[string]interface{}) {
	if spec != nil {
		if cap, ok := spec["capacity"].(map[string]interface{}); ok {
			if storage, ok := cap["storage"].(string); ok {
				ti.Columns = append(ti.Columns, model.KeyValue{Key: "Capacity", Value: storage})
			}
		}
		if am, ok := spec["accessModes"].([]interface{}); ok {
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
		if claimRef, ok := spec["claimRef"].(map[string]interface{}); ok {
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
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Status", Value: phase})
			ti.Status = phase
		}
		if reason, ok := status["reason"].(string); ok && reason != "" {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Reason", Value: reason})
		}
	}
}

func populateResourceQuota(ti *model.Item, status, spec map[string]interface{}) {
	if status != nil {
		hard, _ := status["hard"].(map[string]interface{})
		used, _ := status["used"].(map[string]interface{})
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
		if hard, ok := spec["hard"].(map[string]interface{}); ok {
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

func populateLimitRange(ti *model.Item, spec map[string]interface{}) {
	if spec == nil {
		return
	}
	limits, ok := spec["limits"].([]interface{})
	if !ok {
		return
	}
	for _, l := range limits {
		lMap, ok := l.(map[string]interface{})
		if !ok {
			continue
		}
		lType, _ := lMap["type"].(string)
		prefix := lType
		if prefix == "" {
			prefix = "Unknown"
		}
		if def, ok := lMap["default"].(map[string]interface{}); ok {
			for resource, val := range def {
				ti.Columns = append(ti.Columns, model.KeyValue{
					Key:   fmt.Sprintf("%s Default %s", prefix, resource),
					Value: fmt.Sprintf("%v", val),
				})
			}
		}
		if defReq, ok := lMap["defaultRequest"].(map[string]interface{}); ok {
			for resource, val := range defReq {
				ti.Columns = append(ti.Columns, model.KeyValue{
					Key:   fmt.Sprintf("%s Default Req %s", prefix, resource),
					Value: fmt.Sprintf("%v", val),
				})
			}
		}
		if max, ok := lMap["max"].(map[string]interface{}); ok {
			for resource, val := range max {
				ti.Columns = append(ti.Columns, model.KeyValue{
					Key:   fmt.Sprintf("%s Max %s", prefix, resource),
					Value: fmt.Sprintf("%v", val),
				})
			}
		}
		if min, ok := lMap["min"].(map[string]interface{}); ok {
			for resource, val := range min {
				ti.Columns = append(ti.Columns, model.KeyValue{
					Key:   fmt.Sprintf("%s Min %s", prefix, resource),
					Value: fmt.Sprintf("%v", val),
				})
			}
		}
	}
}

func populatePodDisruptionBudget(ti *model.Item, status, spec map[string]interface{}) {
	if spec != nil {
		if minAvail, ok := spec["minAvailable"]; ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Min Available", Value: fmt.Sprintf("%v", minAvail)})
		}
		if maxUnavail, ok := spec["maxUnavailable"]; ok {
			ti.Columns = append(ti.Columns, model.KeyValue{Key: "Max Unavailable", Value: fmt.Sprintf("%v", maxUnavail)})
		}
		if selector, ok := spec["selector"].(map[string]interface{}); ok {
			if matchLabels, ok := selector["matchLabels"].(map[string]interface{}); ok {
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

func populateNetworkPolicy(ti *model.Item, spec map[string]interface{}) {
	if spec == nil {
		return
	}
	if selector, ok := spec["podSelector"].(map[string]interface{}); ok {
		if matchLabels, ok := selector["matchLabels"].(map[string]interface{}); ok {
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
	if policyTypes, ok := spec["policyTypes"].([]interface{}); ok {
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
	if ingress, ok := spec["ingress"].([]interface{}); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Ingress Rules", Value: fmt.Sprintf("%d", len(ingress))})
	}
	if egress, ok := spec["egress"].([]interface{}); ok {
		ti.Columns = append(ti.Columns, model.KeyValue{Key: "Egress Rules", Value: fmt.Sprintf("%d", len(egress))})
	}
}
