// Package ui - config_kubeconfig.go
package ui

import (
	"encoding/json"
	"strings"

	"github.com/janosmiko/lfk/internal/logger"
)

// ConfigKubeconfigDirs holds the kubeconfig_dir value from the config file (if any).
// The YAML value may be either a single string or a list of strings. Both are
// normalised into this slice. Empty (nil) means "no override, use the default".
var ConfigKubeconfigDirs []string

// ConfigKubeconfigExclusive holds kubeconfig_exclusive from the config file:
// whether a set KUBECONFIG suppresses the default kubeconfig discovery
// (kubectl semantics). Defaults to true. CLI flag and env override it at
// client construction (see k8s.ResolveKubeconfigExclusive).
var ConfigKubeconfigExclusive = true

// ConfigKubeconfigIgnore holds kubeconfig_ignore: the glob patterns discovery
// skips. Nil means "not configured" and yields the default list, while a
// non-nil empty slice is a deliberate "ignore nothing".
var ConfigKubeconfigIgnore []string

// applyKubeconfigIgnoreSetting trims each pattern because filepath.Match reads
// a surrounding space as a literal character, so " *.log " would silently match
// nothing. Entries left empty by the trim are dropped.
func applyKubeconfigIgnoreSetting(patterns *[]string) {
	if patterns == nil {
		return
	}
	out := make([]string, 0, len(*patterns))
	for _, p := range *patterns {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	ConfigKubeconfigIgnore = out
}

// kubeconfigDirsSetting holds the parsed kubeconfig_dir config value. It accepts
// either a string (single directory) or a list of strings (multiple directories
// to merge). Resolved during LoadConfig and stored in ConfigKubeconfigDirs.
//
// UnmarshalJSON is deliberately tolerant: a typo or unsupported shape is
// captured in raw + invalid rather than aborting the whole config load —
// same pattern informer_cache uses, so a bad kubeconfig_dir value never
// silently nukes unrelated keys like keybindings or colorscheme.
type kubeconfigDirsSetting struct {
	paths   []string
	raw     string
	invalid bool
}

// applyKubeconfigDirsSetting writes the resolved paths into
// ConfigKubeconfigDirs, or warns and falls back when the user supplied an
// unrecognised shape. Extracted from applyConfigOptions to keep that
// dispatcher under the project's cyclomatic-complexity cap.
func applyKubeconfigDirsSetting(s *kubeconfigDirsSetting) {
	if s == nil {
		return
	}
	if s.invalid {
		logger.Warn("unrecognised kubeconfig_dir value in config; ignored",
			"value", s.raw,
			"valid", "string or list of strings")
		return
	}
	if len(s.paths) > 0 {
		ConfigKubeconfigDirs = s.paths
	}
}

// UnmarshalJSON parses the string or []string union forms into paths.
// Whitespace-only entries are dropped so a YAML value like
// `kubeconfig_dir: " "` does not silently shadow a CLI flag or env var —
// that mirrors the trimming applied to the env var / CLI surfaces.
//
// LoadConfig goes through sigs.k8s.io/yaml, which converts YAML to JSON
// before unmarshalling, so this hook also handles YAML files.
func (s *kubeconfigDirsSetting) UnmarshalJSON(data []byte) error {
	// String form first.
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		if v := strings.TrimSpace(single); v != "" {
			s.paths = []string{v}
		}
		return nil
	}
	// List form.
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		for _, p := range list {
			if v := strings.TrimSpace(p); v != "" {
				s.paths = append(s.paths, v)
			}
		}
		return nil
	}
	// Truly unparseable shape (number, object, etc.). Capture for warning.
	s.raw = strings.TrimSpace(string(data))
	s.invalid = true
	return nil
}
