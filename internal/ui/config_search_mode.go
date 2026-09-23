// Package ui - config_search_mode.go
// The search_mode setting: the default match type for every search/filter
// input before any ~ or \ prefix override.
package ui

import (
	"strings"

	"github.com/janosmiko/lfk/internal/logger"
)

// Default search mode values, set via the `search_mode:` config key.
const (
	DefaultSearchModeDefault = "default" // substring, auto-regex, ~ fuzzy, \ literal (today's behavior)
	DefaultSearchModeFuzzy   = "fuzzy"   // plain input matches fuzzy; ~ still fuzzy; \ forces literal substring
	DefaultSearchModeRegex   = "regex"   // plain input matches regex; ~ still fuzzy; \ forces literal substring
)

// ConfigDefaultSearchMode selects the default match type DetectSearchMode
// applies to a plain query (no ~ or \ prefix).
var ConfigDefaultSearchMode = DefaultSearchModeDefault

// applySearchMode validates and applies the search_mode config value.
// Empty keeps the compiled default. Unknown values warn and keep it too.
func applySearchMode(raw string) {
	v := strings.ToLower(raw)
	if v == "" {
		return
	}
	switch v {
	case DefaultSearchModeDefault, DefaultSearchModeFuzzy, DefaultSearchModeRegex:
		ConfigDefaultSearchMode = v
	default:
		logger.Warn("Invalid search_mode; using default",
			"accepted", []string{DefaultSearchModeDefault, DefaultSearchModeFuzzy, DefaultSearchModeRegex},
			"default", DefaultSearchModeDefault)
	}
}
