package ui

import "github.com/janosmiko/lfk/internal/model"

// builtinColWidths holds the precomputed widths for the fixed (non-extra)
// columns, indexed by name. Bundling them keeps the dispatcher signatures
// tractable as the set of builtin columns grows.
type builtinColWidths struct {
	context, ns, ready, restarts, status, age int
}

// builtinColHeaders holds the precomputed (already padded + sort-indicator
// decorated) header strings for each fixed column.
type builtinColHeaders struct {
	context, ns, ready, restarts, status, age string
}

// plainCellInputs carries the inputs needed to render a plain-text builtin
// cell. The preprocessed value strings come from the caller because some
// (e.g. restarts) have row-specific upstream handling.
type plainCellInputs struct {
	item                             *model.Item
	ns, ready, restarts, status, age string
	widths                           builtinColWidths
}

// styledCellInputs carries the inputs needed to render a styled builtin cell.
type styledCellInputs struct {
	item             model.Item
	widths           builtinColWidths
	anyRecentRestart bool
}

// builtinColumn is the metadata + render functions for a single fixed
// (non-extra) column. Holding these in a registry lets the four dispatch
// switches (width / header / plain / styled) collapse into one map lookup.
type builtinColumn struct {
	key    string
	width  func(builtinColWidths) int
	header func(builtinColHeaders) string
	plain  func(plainCellInputs) string
	styled func(styledCellInputs) string
}

// builtinColumns is the canonical list of fixed columns. Adding a new builtin
// column means appending one entry here instead of editing five dispatch
// switches.
//
// Context is intentionally part of this registry but is also treated as a
// conditional column: when its precomputed width is zero, the row renderer
// falls through to the extras lookup so a user-requested "Context" extra
// can still render.
var builtinColumns = []builtinColumn{
	{
		key:    "Context",
		width:  func(w builtinColWidths) int { return w.context },
		header: func(h builtinColHeaders) string { return h.context },
		plain: func(p plainCellInputs) string {
			name := ""
			if p.item != nil {
				name = p.item.ClusterName
			}
			return padRight(Truncate(name, p.widths.context-1), p.widths.context)
		},
		styled: func(s styledCellInputs) string {
			return DimStyle.Render(padRight(Truncate(s.item.ClusterName, s.widths.context-1), s.widths.context))
		},
	},
	{
		key:    "Namespace",
		width:  func(w builtinColWidths) int { return w.ns },
		header: func(h builtinColHeaders) string { return h.ns },
		plain: func(p plainCellInputs) string {
			return padRight(Truncate(SanitizeTerminalText(p.ns), p.widths.ns-1), p.widths.ns)
		},
		styled: func(s styledCellInputs) string {
			ns := SanitizeTerminalText(s.item.Namespace)
			if ns == "" {
				ns = "-"
			}
			return DimStyle.Render(padRight(Truncate(ns, s.widths.ns-1), s.widths.ns))
		},
	},
	{
		key:    "Ready",
		width:  func(w builtinColWidths) int { return w.ready },
		header: func(h builtinColHeaders) string { return h.ready },
		plain: func(p plainCellInputs) string {
			return padRight(SanitizeTerminalText(p.ready), p.widths.ready)
		},
		styled: func(s styledCellInputs) string {
			return DimStyle.Render(padRight(SanitizeTerminalText(s.item.Ready), s.widths.ready))
		},
	},
	{
		key:    "Restarts",
		width:  func(w builtinColWidths) int { return w.restarts },
		header: func(h builtinColHeaders) string { return h.restarts },
		plain: func(p plainCellInputs) string {
			return padRight(p.restarts, p.widths.restarts)
		},
		styled: func(s styledCellInputs) string {
			return styledRestartsCell(s.item, s.widths.restarts, s.anyRecentRestart)
		},
	},
	{
		key:    "Status",
		width:  func(w builtinColWidths) int { return w.status },
		header: func(h builtinColHeaders) string { return h.status },
		plain: func(p plainCellInputs) string {
			status := SanitizeTerminalText(p.status)
			return padRight(Truncate(AbbreviateStatusForWidth(status, p.widths.status-1), p.widths.status-1), p.widths.status)
		},
		styled: func(s styledCellInputs) string {
			status := SanitizeTerminalText(s.item.Status)
			val := AbbreviateStatusForWidth(status, s.widths.status-1)
			return StatusStyle(val).Render(padRight(Truncate(val, s.widths.status-1), s.widths.status))
		},
	},
	{
		key:    "Age",
		width:  func(w builtinColWidths) int { return w.age },
		header: func(h builtinColHeaders) string { return h.age },
		plain: func(p plainCellInputs) string {
			return padRight(p.age, p.widths.age)
		},
		styled: func(s styledCellInputs) string {
			age := LiveAge(s.item)
			return AgeStyle(age).Render(padRight(age, s.widths.age))
		},
	},
}

var builtinColumnsByKey = func() map[string]*builtinColumn {
	m := make(map[string]*builtinColumn, len(builtinColumns))
	for i := range builtinColumns {
		m[builtinColumns[i].key] = &builtinColumns[i]
	}
	return m
}()

// renderableBuiltin returns the builtin column entry for key, or nil. Context
// is only renderable when its precomputed width is positive; other builtins
// always count as renderable (callers may still get a zero-width cell). This
// gating lives here so the row renderer doesn't need to special-case Context.
func renderableBuiltin(key string, widths builtinColWidths) *builtinColumn {
	col, ok := builtinColumnsByKey[key]
	if !ok {
		return nil
	}
	if key == "Context" && widths.context <= 0 {
		return nil
	}
	return col
}
