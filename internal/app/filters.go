package app

import (
	"strconv"
	"strings"
	"time"

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
// kind-specific presets are added on top.
func builtinFilterPresets(kind string) []FilterPreset {
	var presets []FilterPreset

	// --- Kind-specific presets (added first so they appear at the top) ---
	switch kind {
	case "Pod":
		presets = append(presets,
			FilterPreset{Name: "Failing", Description: "CrashLoop / Error / ImagePull / OOMKilled", Key: "f",
				MatchFn: func(item model.Item) bool {
					s := strings.ToLower(item.Status)
					return s == "failed" || s == "error" || s == "crashloopbackoff" ||
						s == "imagepullbackoff" || s == "errimagepull" || s == "oomkilled" ||
						s == "evicted" || s == "createcontainerconfigerror"
				}},
			FilterPreset{Name: "Pending", Description: "Pending / ContainerCreating / Terminating", Key: "p",
				MatchFn: func(item model.Item) bool {
					s := strings.ToLower(item.Status)
					return s == "pending" || s == "containercreating" || s == "podinitializing" ||
						s == "init:0/1" || s == "terminating" || s == "unknown"
				}},
			FilterPreset{Name: "Not Ready", Description: "Ready containers mismatch", Key: "n",
				MatchFn: matchReadyMismatch},
			FilterPreset{Name: "Restarting", Description: "Restart count > 0", Key: "r",
				MatchFn: matchRestartsGt(0)},
			FilterPreset{Name: "High Restarts", Description: "Restart count > 10", Key: "R",
				MatchFn: matchRestartsGt(10)},
		)

	case "Deployment", "StatefulSet", "DaemonSet":
		presets = append(presets,
			FilterPreset{Name: "Not Ready", Description: "Ready replicas != desired", Key: "n",
				MatchFn: matchReadyMismatch},
			FilterPreset{Name: "Failing", Description: "Progressing=False or unavailable replicas", Key: "f",
				MatchFn: func(item model.Item) bool {
					s := strings.ToLower(item.Status)
					if s == "failed" || s == "error" || s == "degraded" {
						return true
					}
					// Check for unavailable replicas in columns.
					if ua := columnValue(item, "Unavailable"); ua != "" && ua != "0" {
						return true
					}
					return matchReadyMismatch(item)
				}},
		)

	case "Node":
		presets = append(presets,
			FilterPreset{Name: "Not Ready", Description: "Node status != Ready", Key: "n",
				MatchFn: func(item model.Item) bool {
					s := strings.ToLower(item.Status)
					return s != "ready"
				}},
			FilterPreset{Name: "Cordoned", Description: "SchedulingDisabled", Key: "c",
				MatchFn: func(item model.Item) bool {
					s := strings.ToLower(item.Status)
					return strings.Contains(s, "schedulingdisabled")
				}},
		)

	case "Job":
		presets = append(presets,
			FilterPreset{Name: "Failed", Description: "Job failed or hit BackoffLimit", Key: "f",
				MatchFn: func(item model.Item) bool {
					s := strings.ToLower(item.Status)
					return strings.Contains(s, "failed") || strings.Contains(s, "backofflimit")
				}},
		)

	case "CronJob":
		presets = append(presets,
			FilterPreset{Name: "Suspended", Description: "CronJob is suspended", Key: "s",
				MatchFn: func(item model.Item) bool {
					return strings.EqualFold(columnValue(item, "Suspend"), "true")
				}},
		)

	case "Service":
		presets = append(presets,
			FilterPreset{Name: "LB No IP", Description: "LoadBalancer without external IP", Key: "l",
				MatchFn: func(item model.Item) bool {
					svcType := columnValue(item, "Type")
					if !strings.EqualFold(svcType, "loadbalancer") {
						return false
					}
					ext := columnValue(item, "External-IP")
					return ext == "" || ext == "<none>" || ext == "<pending>"
				}},
		)

	case "Certificate", "CertificateRequest":
		presets = append(presets,
			FilterPreset{Name: "Not Ready", Description: "Certificate not ready", Key: "n",
				MatchFn: func(item model.Item) bool {
					s := strings.ToLower(item.Status)
					return !strings.Contains(s, "true") && s != "ready"
				}},
			FilterPreset{Name: "Expiring Soon", Description: "Expires within 30 days", Key: "e",
				MatchFn: func(item model.Item) bool {
					exp := columnValue(item, "Expires")
					if exp == "" {
						exp = columnValue(item, "Not After")
					}
					if exp == "" {
						return false
					}
					// Try common time formats.
					for _, layout := range []string{
						time.RFC3339,
						"2006-01-02T15:04:05Z",
						"2006-01-02 15:04:05",
						"2006-01-02",
					} {
						if t, err := time.Parse(layout, exp); err == nil {
							return time.Until(t) < 30*24*time.Hour && time.Until(t) > 0
						}
					}
					return false
				}},
		)

	case "Application": // ArgoCD
		presets = append(presets,
			FilterPreset{Name: "Out of Sync", Description: "Sync status is OutOfSync", Key: "s",
				MatchFn: func(item model.Item) bool {
					s := strings.ToLower(item.Status)
					return strings.Contains(s, "outofsync")
				}},
			FilterPreset{Name: "Degraded", Description: "Health is Degraded or Missing", Key: "d",
				MatchFn: func(item model.Item) bool {
					s := strings.ToLower(item.Status)
					return strings.Contains(s, "degraded") || strings.Contains(s, "missing")
				}},
		)

	case "HelmRelease":
		presets = append(presets,
			FilterPreset{Name: "Suspended", Description: "Reconciliation suspended", Key: "s",
				MatchFn: func(item model.Item) bool {
					s := strings.ToLower(item.Status)
					return strings.Contains(s, "suspended")
				}},
			FilterPreset{Name: "Not Ready", Description: "Not in Ready/Applied state", Key: "n",
				MatchFn: func(item model.Item) bool {
					s := strings.ToLower(item.Status)
					return s != "ready" && s != "applied" && !strings.Contains(s, "suspended")
				}},
		)

	case "Kustomization":
		presets = append(presets,
			FilterPreset{Name: "Suspended", Description: "Reconciliation suspended", Key: "s",
				MatchFn: func(item model.Item) bool {
					s := strings.ToLower(item.Status)
					return strings.Contains(s, "suspended")
				}},
			FilterPreset{Name: "Not Ready", Description: "Not in Ready/Applied state", Key: "n",
				MatchFn: func(item model.Item) bool {
					s := strings.ToLower(item.Status)
					return s != "ready" && s != "applied" && !strings.Contains(s, "suspended")
				}},
		)

	case "PersistentVolumeClaim":
		presets = append(presets,
			FilterPreset{Name: "Pending", Description: "PVC not yet bound", Key: "p",
				MatchFn: func(item model.Item) bool {
					return strings.EqualFold(item.Status, "pending")
				}},
			FilterPreset{Name: "Lost", Description: "PVC lost its backing volume", Key: "l",
				MatchFn: func(item model.Item) bool {
					return strings.EqualFold(item.Status, "lost")
				}},
		)

	case "Event":
		presets = append(presets,
			FilterPreset{Name: "Warnings", Description: "Warning events only", Key: "w",
				MatchFn: func(item model.Item) bool {
					return strings.EqualFold(item.Status, "warning")
				}},
		)
	}

	// --- Universal presets (shown for all kinds) ---
	presets = append(presets,
		FilterPreset{Name: "Old (>30d)", Description: "Resources older than 30 days", Key: "o",
			MatchFn: func(item model.Item) bool {
				if item.CreatedAt.IsZero() {
					return false
				}
				return time.Since(item.CreatedAt) > 30*24*time.Hour
			}},
		FilterPreset{Name: "Recent (<1h)", Description: "Resources created in the last hour", Key: "h",
			MatchFn: func(item model.Item) bool {
				if item.CreatedAt.IsZero() {
					return false
				}
				return time.Since(item.CreatedAt) < time.Hour
			}},
	)

	// --- User-configured presets from config file ---
	presets = appendConfigPresets(presets, kind)

	return presets
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
