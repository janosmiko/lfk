// Package ui - config_row_tint.go
// The row_status_tint setting: whole-row emphasis for failed/progressing
// statuses (issue #540).
package ui

import (
	"strings"

	"github.com/janosmiko/lfk/internal/logger"
)

// Row status tint modes: how list rows whose status classifies as failed or
// progressing are emphasized beyond the Status cell.
const (
	RowStatusTintOff        = "off"        // status cell only (pre-#540 behavior)
	RowStatusTintForeground = "foreground" // whole row text in the severity color
	RowStatusTintBackground = "background" // muted severity background across the row
)

// ConfigRowStatusTint selects the row emphasis mode for failed/progressing
// statuses. Set via the `row_status_tint:` config key.
var ConfigRowStatusTint = RowStatusTintForeground

// applyRowStatusTint validates and applies the row_status_tint config value.
// Empty keeps the compiled default; unknown values warn and keep it too.
func applyRowStatusTint(raw string) {
	v := strings.ToLower(raw)
	if v == "" {
		return
	}
	switch v {
	case RowStatusTintOff, RowStatusTintForeground, RowStatusTintBackground:
		ConfigRowStatusTint = v
	default:
		logger.Warn("Invalid row_status_tint; using default",
			"accepted", []string{RowStatusTintOff, RowStatusTintForeground, RowStatusTintBackground},
			"default", RowStatusTintForeground)
	}
}
