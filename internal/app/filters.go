package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// FilterPreset defines a quick filter that can be applied to the resource list.
type FilterPreset struct {
	Name        string
	Description string
	Key         string // shortcut key in the filter preset overlay
	MatchFn     func(item model.Item) bool
}

// columnValue returns the value of the first Columns entry matching the given
// key (case-insensitive). Returns "" if not found.
func columnValue(item model.Item, key string) string {
	lower := strings.ToLower(key)
	for _, kv := range item.Columns {
		if strings.ToLower(kv.Key) == lower {
			return kv.Value
		}
	}
	return ""
}

// builtinFilterPresets returns the quick filter presets relevant to the given
// resource kind. Universal presets (Old, Recent) are included for every kind;
// kind-specific presets are added on top. Orphan presets are not included
// because this shim has no cache reference; callers that have a Model use
// builtinFilterPresetsWithOrphans instead.
func builtinFilterPresets(kind string) []FilterPreset {
	return builtinFilterPresetsWithOrphans(kind, nil, orphanCacheKey{})
}

// builtinFilterPresetsWithOrphans is the variant the runtime uses; the orphan
// presets are appended only when a non-nil cache is supplied so unit tests
// that lack a Model still compile and the original call sites keep working.
func builtinFilterPresetsWithOrphans(kind string, cache map[orphanCacheKey]*k8s.OrphanReport, key orphanCacheKey) []FilterPreset {
	presets := kindFilterPresets(kind)

	if cache != nil {
		presets = append(presets, orphanPresetsForKind(kind, cache, key)...)
	}

	// --- Universal presets (shown for all kinds) ---
	presets = append(presets,
		FilterPreset{
			Name: "Old (>30d)", Description: "Resources older than 30 days", Key: "o",
			MatchFn: func(item model.Item) bool {
				if item.CreatedAt.IsZero() {
					return false
				}
				return time.Since(item.CreatedAt) > 30*24*time.Hour
			},
		},
		FilterPreset{
			Name: "Recent (<1h)", Description: "Resources created in the last hour", Key: "h",
			MatchFn: func(item model.Item) bool {
				if item.CreatedAt.IsZero() {
					return false
				}
				return time.Since(item.CreatedAt) < time.Hour
			},
		},
	)

	// --- User-configured presets from config file ---
	presets = appendConfigPresets(presets, kind)

	return presets
}

// kindFilterPresets returns the kind-specific filter presets.
func kindFilterPresets(kind string) []FilterPreset {
	switch kind {
	case "Pod":
		return podFilterPresets()
	case "Deployment", "StatefulSet", "DaemonSet":
		return workloadFilterPresets()
	case "Node":
		return nodeFilterPresets()
	case "Job":
		return jobFilterPresets()
	case "CronJob":
		return cronjobFilterPresets()
	case "Service":
		return serviceFilterPresets()
	case "Certificate", "CertificateRequest":
		return certFilterPresets()
	case "Application":
		return argoFilterPresets()
	case "HelmRelease", "Kustomization":
		return fluxFilterPresets()
	case "PersistentVolumeClaim":
		return pvcFilterPresets()
	case "Event":
		return eventFilterPresets()
	default:
		return nil
	}
}

func podFilterPresets() []FilterPreset {
	return []FilterPreset{
		{
			Name: "Failing", Description: "CrashLoop / Error / ImagePull / OOMKilled", Key: "f",
			MatchFn: func(item model.Item) bool {
				s := strings.ToLower(item.Status)
				return s == "failed" || s == "error" || s == "crashloopbackoff" ||
					s == "imagepullbackoff" || s == "errimagepull" || s == "oomkilled" ||
					s == "evicted" || s == "createcontainerconfigerror"
			},
		},
		{
			Name: "Pending", Description: "Pending / ContainerCreating / Terminating", Key: "p",
			MatchFn: func(item model.Item) bool {
				s := strings.ToLower(item.Status)
				return s == "pending" || s == "containercreating" || s == "podinitializing" ||
					s == "init:0/1" || s == "terminating" || s == "unknown"
			},
		},
		{Name: "Not Ready", Description: "Ready containers mismatch", Key: "n", MatchFn: matchReadyMismatch},
		{Name: "Restarting", Description: "Restart count > 0", Key: "r", MatchFn: matchRestartsGt(0)},
		{Name: "High Restarts", Description: "Restart count > 10", Key: "R", MatchFn: matchRestartsGt(10)},
	}
}

func workloadFilterPresets() []FilterPreset {
	return []FilterPreset{
		{Name: "Not Ready", Description: "Ready replicas != desired", Key: "n", MatchFn: matchReadyMismatch},
		{
			Name: "Failing", Description: "Progressing=False or unavailable replicas", Key: "f",
			MatchFn: func(item model.Item) bool {
				s := strings.ToLower(item.Status)
				if s == "failed" || s == "error" || s == "degraded" {
					return true
				}
				if ua := columnValue(item, "Unavailable"); ua != "" && ua != "0" {
					return true
				}
				return matchReadyMismatch(item)
			},
		},
	}
}

func nodeFilterPresets() []FilterPreset {
	return []FilterPreset{
		{
			Name: "Not Ready", Description: "Node status != Ready", Key: "n",
			MatchFn: func(item model.Item) bool { return strings.ToLower(item.Status) != "ready" },
		},
		{
			Name: "Cordoned", Description: "SchedulingDisabled", Key: "c",
			MatchFn: func(item model.Item) bool {
				return strings.Contains(strings.ToLower(item.Status), "schedulingdisabled")
			},
		},
	}
}

func jobFilterPresets() []FilterPreset {
	return []FilterPreset{
		{
			Name: "Failed", Description: "Job failed or hit BackoffLimit", Key: "f",
			MatchFn: func(item model.Item) bool {
				s := strings.ToLower(item.Status)
				return strings.Contains(s, "failed") || strings.Contains(s, "backofflimit")
			},
		},
	}
}

func cronjobFilterPresets() []FilterPreset {
	return []FilterPreset{
		{
			Name: "Suspended", Description: "CronJob is suspended", Key: "s",
			MatchFn: func(item model.Item) bool { return strings.EqualFold(columnValue(item, "Suspend"), "true") },
		},
	}
}

func serviceFilterPresets() []FilterPreset {
	return []FilterPreset{
		{
			Name: "LB No IP", Description: "LoadBalancer without external IP", Key: "l",
			MatchFn: func(item model.Item) bool {
				if !strings.EqualFold(columnValue(item, "Type"), "loadbalancer") {
					return false
				}
				ext := columnValue(item, "External-IP")
				return ext == "" || ext == "<none>" || ext == "<pending>"
			},
		},
	}
}

func certFilterPresets() []FilterPreset {
	return []FilterPreset{
		{
			Name: "Not Ready", Description: "Certificate not ready", Key: "n",
			MatchFn: func(item model.Item) bool {
				s := strings.ToLower(item.Status)
				return !strings.Contains(s, "true") && s != "ready"
			},
		},
		{
			Name: "Expiring Soon", Description: "Expires within 30 days", Key: "e",
			MatchFn: func(item model.Item) bool {
				exp := columnValue(item, "Expires")
				if exp == "" {
					exp = columnValue(item, "Not After")
				}
				if exp == "" {
					return false
				}
				for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05", "2006-01-02"} {
					if t, err := time.Parse(layout, exp); err == nil {
						return time.Until(t) < 30*24*time.Hour && time.Until(t) > 0
					}
				}
				return false
			},
		},
	}
}

func argoFilterPresets() []FilterPreset {
	return []FilterPreset{
		{
			Name: "Out of Sync", Description: "Sync status is OutOfSync", Key: "s",
			MatchFn: func(item model.Item) bool { return strings.Contains(strings.ToLower(item.Status), "outofsync") },
		},
		{
			Name: "Degraded", Description: "Health is Degraded or Missing", Key: "d",
			MatchFn: func(item model.Item) bool {
				s := strings.ToLower(item.Status)
				return strings.Contains(s, "degraded") || strings.Contains(s, "missing")
			},
		},
	}
}

func fluxFilterPresets() []FilterPreset {
	return []FilterPreset{
		{
			Name: "Suspended", Description: "Reconciliation suspended", Key: "s",
			MatchFn: func(item model.Item) bool { return strings.Contains(strings.ToLower(item.Status), "suspended") },
		},
		{
			Name: "Not Ready", Description: "Not in Ready/Applied state", Key: "n",
			MatchFn: func(item model.Item) bool {
				s := strings.ToLower(item.Status)
				return s != "ready" && s != "applied" && !strings.Contains(s, "suspended")
			},
		},
	}
}

func pvcFilterPresets() []FilterPreset {
	return []FilterPreset{
		{
			Name: "Pending", Description: "PVC not yet bound", Key: "p",
			MatchFn: func(item model.Item) bool { return strings.EqualFold(item.Status, "pending") },
		},
		{
			Name: "Lost", Description: "PVC lost its backing volume", Key: "l",
			MatchFn: func(item model.Item) bool { return strings.EqualFold(item.Status, "lost") },
		},
	}
}

func eventFilterPresets() []FilterPreset {
	return []FilterPreset{
		{
			Name: "Warnings", Description: "Warning events only", Key: "w",
			MatchFn: func(item model.Item) bool { return strings.EqualFold(item.Status, "warning") },
		},
	}
}

// matchReadyMismatch returns true when the item's Ready field has a numerator
// that does not equal the denominator (e.g., "1/3").
func matchReadyMismatch(item model.Item) bool {
	if item.Ready == "" {
		return false
	}
	parts := strings.SplitN(item.Ready, "/", 2)
	if len(parts) == 2 {
		return parts[0] != parts[1]
	}
	return false
}

// matchRestartsGt returns a MatchFn that is true when item.Restarts > threshold.
func matchRestartsGt(threshold int) func(model.Item) bool {
	return func(item model.Item) bool {
		if item.Restarts == "" {
			return false
		}
		n, err := strconv.Atoi(strings.TrimSpace(item.Restarts))
		if err != nil {
			return false
		}
		return n > threshold
	}
}

// appendConfigPresets converts user-configured filter presets for the given kind
// and appends them to the preset list.
func appendConfigPresets(presets []FilterPreset, kind string) []FilterPreset {
	if len(ui.ConfigFilterPresets) == 0 {
		return presets
	}

	// Look up presets for the exact kind (case-insensitive).
	kindLower := strings.ToLower(kind)
	cfgPresets, ok := ui.ConfigFilterPresets[kindLower]
	if !ok {
		return presets
	}

	// Collect existing shortcut keys to avoid collisions.
	usedKeys := make(map[string]bool, len(presets))
	for _, p := range presets {
		usedKeys[p.Key] = true
	}

	for _, cp := range cfgPresets {
		key := cp.Key
		if key == "" || usedKeys[key] {
			// Skip presets with no key or duplicate keys.
			continue
		}
		usedKeys[key] = true
		presets = append(presets, FilterPreset{
			Name:        cp.Name,
			Description: cp.Name, // use name as description if none provided
			Key:         key,
			MatchFn:     buildConfigMatchFn(cp.Match),
		})
	}

	return presets
}

// orphanMatcher builds a FilterPreset.MatchFn that returns true when the item
// is in the cached OrphanReport for the given (key, kind). An empty cache or a
// missing key causes the function to return false for everything; the load
// command fires asynchronously when the preset is activated, then a re-render
// populates the list.
func orphanMatcher(cache map[orphanCacheKey]*k8s.OrphanReport, key orphanCacheKey, kind string) func(model.Item) bool {
	return func(item model.Item) bool {
		report := cache[key]
		if report == nil {
			return false
		}
		var pool []k8s.OrphanItem
		switch kind {
		case "Pod":
			pool = report.Pods
		case "Secret":
			pool = report.Secrets
		case "ConfigMap":
			pool = report.ConfigMaps
		case "Service":
			pool = report.Services
		default:
			return false
		}
		for _, o := range pool {
			if o.Namespace == item.Namespace && o.Name == item.Name {
				return true
			}
		}
		return false
	}
}

// orphanPresetsForKind returns the orphan filter presets relevant to the given
// kind.
func orphanPresetsForKind(kind string, cache map[orphanCacheKey]*k8s.OrphanReport, key orphanCacheKey) []FilterPreset {
	switch kind {
	case "Pod":
		return []FilterPreset{{
			Name: "Orphans", Description: "Pods with no owner reference", Key: "O",
			MatchFn: orphanMatcher(cache, key, "Pod"),
		}}
	case "Secret":
		return []FilterPreset{{
			Name: "Unmounted", Description: "Not referenced by any Pod / workload template / Ingress / ServiceAccount", Key: "u",
			MatchFn: orphanMatcher(cache, key, "Secret"),
		}}
	case "ConfigMap":
		return []FilterPreset{{
			Name: "Unmounted", Description: "Not referenced by any Pod or workload template (Deployment / Job / CronJob / …)", Key: "u",
			MatchFn: orphanMatcher(cache, key, "ConfigMap"),
		}}
	case "Service":
		return []FilterPreset{{
			Name: "No Endpoints", Description: "Service has zero ready+notReady endpoints", Key: "e",
			MatchFn: orphanMatcher(cache, key, "Service"),
		}}
	}
	return nil
}

// needsOrphanCache reports whether opening the filter-preset overlay for this
// kind should kick off a DetectOrphans scan. Pod/Secret/ConfigMap/Service have
// orphan presets whose MatchFn reads from orphanCache; other kinds skip the
// load.
func needsOrphanCache(kind string) bool {
	switch kind {
	case "Pod", "Secret", "ConfigMap", "Service":
		return true
	}
	return false
}

// buildConfigMatchFn converts a ConfigFilterMatch into a MatchFn closure.
func buildConfigMatchFn(m ui.ConfigFilterMatch) func(model.Item) bool {
	return func(item model.Item) bool {
		// All non-zero fields must match (AND logic).
		if m.Status != "" {
			if !strings.Contains(strings.ToLower(item.Status), strings.ToLower(m.Status)) {
				return false
			}
		}
		if m.ReadyNot {
			if !matchReadyMismatch(item) {
				return false
			}
		}
		if m.RestartsGt > 0 {
			n, err := strconv.Atoi(strings.TrimSpace(item.Restarts))
			if err != nil || n <= m.RestartsGt {
				return false
			}
		}
		if m.Column != "" {
			val := columnValue(item, m.Column)
			if m.ColumnValue != "" {
				if !strings.Contains(strings.ToLower(val), strings.ToLower(m.ColumnValue)) {
					return false
				}
			} else {
				// If column is specified without a value, match when column is non-empty.
				if val == "" {
					return false
				}
			}
		}
		return true
	}
}
