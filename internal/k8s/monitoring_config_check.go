package k8s

import (
	"sort"
	"strings"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// UnmatchedMonitoringKeys returns the monitoring config keys, other than
// "_global", that name no known context. Sorted so the warning is stable.
func UnmatchedMonitoringKeys(cfg map[string]model.MonitoringConfig, contexts []string) []string {
	known := make(map[string]struct{}, len(contexts))
	for _, c := range contexts {
		known[c] = struct{}{}
	}
	var out []string
	for key := range cfg {
		if key == "_global" {
			continue
		}
		if _, ok := known[key]; !ok {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// WarnUnmatchedMonitoringKeys logs one warning per monitoring config key
// that matches no kubeconfig context, and returns those keys. A mistyped
// key otherwise fails silently: the "_global" entry applies instead (#705).
func (c *Client) WarnUnmatchedMonitoringKeys() []string {
	items, err := c.GetContexts()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Name)
	}
	unmatched := UnmatchedMonitoringKeys(model.ConfigMonitoring, names)
	for _, key := range unmatched {
		logger.Warn("monitoring config key matches no kubeconfig context; the _global entry applies to that cluster instead",
			"key", key, "contexts", strings.Join(names, ", "))
	}
	return unmatched
}
