package falco

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/janosmiko/lfk/internal/security"
)

// falcoLogEntry represents a single JSON line from Falco's stdout output.
type falcoLogEntry struct {
	Output       string         `json:"output"`
	Priority     string         `json:"priority"`
	Rule         string         `json:"rule"`
	Time         string         `json:"time"`
	Source       string         `json:"source"`
	Tags         []string       `json:"tags"`
	OutputFields map[string]any `json:"output_fields"`
}

// parseLogLine parses a single JSON log line from Falco stdout.
// Returns nil for non-alert lines (info messages, startup logs, etc.).
// When namespace is non-empty, only findings matching that namespace are returned.
func parseLogLine(line []byte, namespace string) []security.Finding {
	var entry falcoLogEntry
	if err := json.Unmarshal(line, &entry); err != nil {
		return nil // not a JSON alert line
	}
	if entry.Rule == "" {
		return nil // not an alert
	}

	// Extract resource reference from output_fields.
	ref := extractRefFromOutputFields(entry.OutputFields)
	if namespace != "" && ref.Namespace != "" && ref.Namespace != namespace {
		return nil // filtered out by namespace
	}

	// Build a stable ID from the rule + resource + time.
	h := sha256.Sum256([]byte(entry.Rule + ref.Namespace + ref.Kind + ref.Name + entry.Time))
	id := "falco/log/" + hex.EncodeToString(h[:8])

	labels := map[string]string{
		"rule":     entry.Rule,
		"priority": strings.ToLower(entry.Priority),
	}
	if entry.Source != "" {
		labels["falco_source"] = entry.Source
	}
	for _, tag := range entry.Tags {
		labels["tag:"+tag] = "true"
	}

	// Build a structured summary from output_fields.
	summary, details := formatFalcoOutput(entry)

	return []security.Finding{
		{
			ID:       id,
			Source:   "falco",
			Category: security.CategoryPolicy,
			Severity: parseSeverity(entry.Priority),
			Title:    entry.Rule,
			Resource: ref,
			Summary:  summary,
			Details:  details,
			Labels:   labels,
		},
	}
}

// formatFalcoOutput extracts a human-readable summary and structured details
// from a Falco log entry. The summary is a one-liner; the details show
// key fields as labeled lines.
func formatFalcoOutput(entry falcoLogEntry) (summary, details string) {
	fields := entry.OutputFields
	str := func(key string) string {
		v, _ := fields[key].(string)
		if v == "" {
			if raw, ok := fields[key]; ok && raw != nil {
				return fmt.Sprintf("%v", raw)
			}
		}
		return v
	}

	// Extract the message part before the | separator.
	summary = entry.Output
	if idx := strings.Index(summary, "|"); idx > 0 {
		summary = strings.TrimSpace(summary[:idx])
	}
	// Strip the timestamp prefix if present.
	if len(summary) > 20 && summary[2] == ':' && summary[5] == ':' {
		if spaceIdx := strings.Index(summary, " "); spaceIdx > 0 {
			rest := strings.TrimSpace(summary[spaceIdx+1:])
			// Skip the priority word (Critical/Warning/etc.)
			if wordEnd := strings.Index(rest, " "); wordEnd > 0 {
				summary = strings.TrimSpace(rest[wordEnd+1:])
			}
		}
	}

	// Build structured details from important fields. Falco uses
	// dot-separated keys in output_fields (proc.exepath, proc.cmdline)
	// but the text output uses underscores (proc_exepath, proc_cmdline).
	// Try both variants for each field.
	var b strings.Builder
	kv := func(label, value string) {
		if value == "" || value == "<NA>" {
			return
		}
		if len(label) < 16 {
			label += strings.Repeat(" ", 16-len(label))
		}
		fmt.Fprintf(&b, "%s  %s\n", label, value)
	}
	// Helper: try multiple keys, return first non-empty.
	any := func(keys ...string) string {
		for _, k := range keys {
			if v := str(k); v != "" && v != "<NA>" {
				return v
			}
		}
		return ""
	}
	kv("Process:", any("proc.exepath", "proc_exepath", "proc.name", "process"))
	kv("Command:", any("proc.cmdline", "proc_cmdline", "command"))
	kv("Parent:", any("proc.pname", "parent", "proc_sname"))
	kv("User:", any("user.name", "user"))
	kv("User UID:", any("user.uid", "user_uid"))
	kv("Container:", any("container.name", "container_name"))
	kv("Image:", any("container.image.repository", "container_image_repository"))
	kv("Image Tag:", any("container.image.tag", "container_image_tag"))
	kv("Container ID:", any("container.id", "container_id"))
	kv("Pod:", any("k8s.pod.name", "k8s_pod_name"))
	kv("Namespace:", any("k8s.ns.name", "k8s_ns_name"))
	kv("Deployment:", any("k8s.deployment.name"))
	kv("Event Type:", any("evt.type", "evt_type"))
	kv("File:", any("fd.name", "fd_name"))
	kv("Directory:", any("proc.cwd", "proc_cwd"))
	kv("Exe Flags:", any("evt.arg.flags", "exe_flags"))
	kv("Time:", entry.Time)

	details = b.String()
	return summary, details
}

// extractRefFromOutputFields reads k8s resource info from Falco's
// output_fields map. Common field names: k8s.ns.name, k8s.pod.name,
// k8s.deployment.name, container.name.
func extractRefFromOutputFields(fields map[string]any) security.ResourceRef {
	str := func(key string) string {
		v, _ := fields[key].(string)
		if v != "" {
			return v
		}
		// Falco sometimes serializes values as non-string types.
		if raw, ok := fields[key]; ok {
			return fmt.Sprintf("%v", raw)
		}
		return ""
	}
	ref := security.ResourceRef{
		Namespace: str("k8s.ns.name"),
	}
	// Try pod first, then higher-level workloads.
	if name := str("k8s.pod.name"); name != "" && name != "<NA>" {
		ref.Kind = "Pod"
		ref.Name = name
		ref.Container = str("container.name")
	} else if name := str("k8s.deployment.name"); name != "" && name != "<NA>" {
		ref.Kind = "Deployment"
		ref.Name = name
	} else if name := str("k8s.daemonset.name"); name != "" && name != "<NA>" {
		ref.Kind = "DaemonSet"
		ref.Name = name
	} else if name := str("k8s.statefulset.name"); name != "" && name != "<NA>" {
		ref.Kind = "StatefulSet"
		ref.Name = name
	} else if name := str("k8s.rc.name"); name != "" && name != "<NA>" {
		ref.Kind = "ReplicaSet"
		ref.Name = name
	}
	return ref
}

// parseEvent converts a Kubernetes Event (generated by falco/falcosidekick)
// into security findings. A single event produces one finding.
func parseEvent(ev *corev1.Event) []security.Finding {
	if ev == nil {
		return nil
	}

	// Extract the Falco rule name from the event reason or annotations.
	rule := ev.Reason
	if rule == "" {
		rule = "unknown"
	}

	// Priority is often in annotations or can be derived from event type.
	priority := ev.Annotations["falco.org/priority"]
	if priority == "" {
		priority = ev.Annotations["priority"]
	}
	if priority == "" {
		// Fall back: Warning events = medium, Normal = low.
		if ev.Type == "Warning" {
			priority = "WARNING"
		} else {
			priority = "NOTICE"
		}
	}

	// Build the resource reference from the involved object.
	ref := security.ResourceRef{
		Namespace: ev.InvolvedObject.Namespace,
		Kind:      ev.InvolvedObject.Kind,
		Name:      ev.InvolvedObject.Name,
	}
	if ref.Namespace == "" {
		ref.Namespace = ev.Namespace
	}

	// Title: use the rule name, cleaned up.
	title := rule

	// Summary: the event message.
	summary := ev.Message

	// Extract Falco-specific labels from annotations.
	labels := map[string]string{
		"rule": rule,
	}
	if v := ev.Annotations["falco.org/rule"]; v != "" {
		labels["rule"] = v
		title = v
	}
	if v := ev.Annotations["falco.org/source"]; v != "" {
		labels["falco_source"] = v
	}
	if v := ev.Annotations["falco.org/output"]; v != "" {
		summary = v
	}
	labels["priority"] = strings.ToLower(priority)

	id := fmt.Sprintf("falco/%s/%s", ev.Namespace, string(ev.UID))

	return []security.Finding{
		{
			ID:       id,
			Source:   "falco",
			Category: security.CategoryPolicy,
			Severity: parseSeverity(priority),
			Title:    title,
			Resource: ref,
			Summary:  summary,
			Labels:   labels,
		},
	}
}
