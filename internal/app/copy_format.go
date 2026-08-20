package app

import (
	"github.com/janosmiko/lfk/internal/model"
)

// CopyFormat enumerates the output formats offered by the Y-key
// copy-as picker. Order is the picker's display order — keep YAML
// first since it's the most kubectl-friendly default.
type CopyFormat int

const (
	CopyFormatYAML CopyFormat = iota
	CopyFormatJSON
	CopyFormatTable
)

// Label returns the title-case display string ("YAML", "JSON",
// "Table"). Used by the picker UI and the clipboard status text.
func (f CopyFormat) Label() string {
	switch f {
	case CopyFormatYAML:
		return "YAML"
	case CopyFormatJSON:
		return "JSON"
	case CopyFormatTable:
		return "Table"
	}
	return ""
}

// ShortcutKey returns the single-letter shortcut that selects this
// format directly from the picker. JSON uses uppercase "J" because
// lowercase "j" is reserved for cursor-down navigation in the picker
// (and globally); the chip-in-picker behaviour stays consistent with
// YAML and Table so every row advertises a shortcut.
func (f CopyFormat) ShortcutKey() string {
	switch f {
	case CopyFormatYAML:
		return "y"
	case CopyFormatJSON:
		return "J"
	case CopyFormatTable:
		return "t"
	}
	return ""
}

// availableCopyFormats returns the picker rows applicable at the
// given navigation level. Clusters and ResourceTypes only support
// Table (there is no manifest behind those rows). All other levels
// offer the full YAML / JSON / Table set.
func availableCopyFormats(level model.Level) []CopyFormat {
	switch level {
	case model.LevelClusters, model.LevelResourceTypes:
		return []CopyFormat{CopyFormatTable}
	default:
		return []CopyFormat{CopyFormatYAML, CopyFormatJSON, CopyFormatTable}
	}
}
