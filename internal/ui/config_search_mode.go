// Package ui - config_search_mode.go
// The search_mode setting: the default match type for every search/filter
// input before any ~ or \ prefix override.
package ui

import (
	"slices"
	"strings"

	"github.com/janosmiko/lfk/internal/logger"
)

// Default search mode values, set via the `search_mode:` config key.
const (
	DefaultSearchModeAuto    = "auto"    // substring, regex when the query has a metacharacter
	DefaultSearchModeLiteral = "literal" // substring only, metacharacters match themselves
	DefaultSearchModeFuzzy   = "fuzzy"
	DefaultSearchModeRegex   = "regex"
)

var searchModes = []string{DefaultSearchModeAuto, DefaultSearchModeLiteral, DefaultSearchModeFuzzy, DefaultSearchModeRegex}

// ConfigDefaultSearchMode selects the default match type DetectSearchMode
// applies to a plain query (no ~ or \ prefix).
var ConfigDefaultSearchMode = DefaultSearchModeAuto

// applySearchMode validates and applies the search_mode config value.
// Empty keeps the compiled default. Unknown values warn and keep it too.
func applySearchMode(raw string) {
	v := strings.ToLower(raw)
	if v == "" {
		return
	}
	if slices.Contains(searchModes, v) {
		ConfigDefaultSearchMode = v
		return
	}
	logger.Warn("Invalid search_mode; using default",
		"accepted", searchModes,
		"default", DefaultSearchModeAuto)
}
