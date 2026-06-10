package ui

// Security-dashboard configuration globals and per-context resolvers. Extracted
// from config.go to keep that file under the 800-line cap.

// ConfigSecurityEnabled is the global default for the security dashboard.
// When false the Security category, SEC badge, and source probing are off.
var ConfigSecurityEnabled = true

// ConfigSecurityHideBadges is the startup default for hiding the per-resource
// SEC severity badge. The dashboard still runs and findings are still scanned;
// only the inline row badge is suppressed. Users can flip it at runtime with
// kb.SecurityBadgeToggle.
var ConfigSecurityHideBadges = false

// ConfigSecuritySources holds global per-source enable/disable overrides,
// keyed by friendly name or internal source id. A source absent from the map
// defaults to enabled.
var ConfigSecuritySources = map[string]bool{}

// ConfigSecurityIgnorePatterns holds the declarative, glob-based security
// ignore rules from the `security.ignore_patterns` config section. Read-only
// after load; consumed by the app layer's ignore checker alongside the
// interactive per-cluster ignore-list.
var ConfigSecurityIgnorePatterns []SecurityIgnorePattern

// ConfigSecuritySecretEnvInclude / ConfigSecuritySecretEnvExclude hold the
// `security.secret_env_include` / `_exclude` env-var name globs that tune the
// heuristic source's secret_env check. Read-only after load.
var (
	ConfigSecuritySecretEnvInclude []string
	ConfigSecuritySecretEnvExclude []string
)

// ConfigClusterSecurityEnabled maps context names to per-cluster
// dashboard-enabled overrides; a value here wins over ConfigSecurityEnabled.
var ConfigClusterSecurityEnabled = map[string]bool{}

// ConfigClusterSecurityHideBadges maps context names to per-cluster
// hide-badges overrides; a value here wins over ConfigSecurityHideBadges.
var ConfigClusterSecurityHideBadges = map[string]bool{}

// ConfigClusterSecuritySources maps context names to per-cluster per-source
// overrides; an entry here wins over ConfigSecuritySources for that context.
var ConfigClusterSecuritySources = map[string]map[string]bool{}

// securitySourceConfigKey maps internal source ids to their friendly config
// key so a `sources:` map written with friendly names resolves correctly.
var securitySourceConfigKey = map[string]string{
	"trivy-operator": "trivy",
	"policy-report":  "kyverno",
}

// ResolveSecurityEnabled returns whether the security dashboard is enabled for
// a context. Precedence: per-context config > global config.
func ResolveSecurityEnabled(context string) bool {
	if v, ok := ConfigClusterSecurityEnabled[context]; ok {
		return v
	}
	return ConfigSecurityEnabled
}

// ResolveSecurityHideBadges returns whether the per-resource SEC badge is
// hidden by default for a context. Precedence: per-context config > global
// config. The runtime toggle layers on top in the app layer.
func ResolveSecurityHideBadges(context string) bool {
	if v, ok := ConfigClusterSecurityHideBadges[context]; ok {
		return v
	}
	return ConfigSecurityHideBadges
}

// securitySourceToggle looks up a source's toggle in a sources map, accepting
// either the internal id or the friendly alias as the key.
func securitySourceToggle(sources map[string]bool, internalName string) (bool, bool) {
	if v, ok := sources[internalName]; ok {
		return v, true
	}
	if key, ok := securitySourceConfigKey[internalName]; ok {
		if v, ok := sources[key]; ok {
			return v, true
		}
	}
	return false, false
}

// ResolveSecuritySourceEnabled returns whether a specific source (by internal
// id, e.g. "trivy-operator") is enabled for a context. Precedence: per-context
// source override > global source override > enabled by default.
func ResolveSecuritySourceEnabled(context, internalName string) bool {
	if m, ok := ConfigClusterSecuritySources[context]; ok {
		if v, found := securitySourceToggle(m, internalName); found {
			return v
		}
	}
	if v, found := securitySourceToggle(ConfigSecuritySources, internalName); found {
		return v
	}
	return true
}
