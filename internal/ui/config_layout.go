// Package ui - config_layout.go
// The appearance.layout setting: the default explorer layout for new tabs.
package ui

import (
	"strings"

	"github.com/janosmiko/lfk/internal/logger"
)

// applyExplorerLayout validates and applies the appearance.layout config
// value. Empty and unknown values both reset to LayoutNormal, not just leave
// it, so a reload that drops the key doesn't keep a stale prior layout.
func applyExplorerLayout(raw string) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case LayoutNormal, LayoutSidebarHidden, LayoutFullscreen:
		ConfigExplorerLayout = v
	case "":
		ConfigExplorerLayout = LayoutNormal
	default:
		ConfigExplorerLayout = LayoutNormal
		logger.Warn("Invalid appearance.layout; using default",
			"accepted", []string{LayoutNormal, LayoutSidebarHidden, LayoutFullscreen},
			"default", LayoutNormal)
	}
}
