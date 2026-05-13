package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// OverlayListFloor is the minimum overlay box width returned by
// OverlayListWidth. Mirrors the historical 70-cell floor used by
// RenderActionOverlay so short menus keep their stable size.
const OverlayListFloor = 70

// overlayListChrome is the horizontal chrome OverlayStyle adds outside
// the content area: 1+1 cell border + 2+2 cell padding.
const overlayListChrome = 6

// OverlayListItem is a single row in the unified overlay list. Optional
// fields render only when the matching feature flag in OverlayListConfig
// is enabled — callers leave them zero-valued otherwise.
type OverlayListItem struct {
	Key         string // shortcut letter; rendered "[k]" when cfg.ShowKey
	Name        string // primary text
	Description string // dim secondary suffix; rendered when cfg.ShowDescription
	Status      string // status badge; rendered "[s]" when cfg.ShowStatus
	Active      bool   // ✓ marker when cfg.ShowActiveMarker
	Selected    bool   // checkbox state when cfg.MultiSelect
	Disabled    bool   // renders dim and (by convention) can't be acted on
}

// OverlayListConfig is the rendering contract for OverlayList. Feature
// flags are independent; set the ones that apply to your overlay shape.
// Items are expected to be pre-filtered by the caller — Filter/FilterActive
// only drive the filter-line display.
type OverlayListConfig struct {
	Title    string
	Subtitle string // optional dim line under the title

	Cursor       int
	Filter       string
	FilterActive bool

	// Feature flags — render the matching item field/marker when true.
	MultiSelect      bool
	ShowActiveMarker bool
	ShowKey          bool
	ShowStatus       bool
	ShowDescription  bool

	// Scroll window into Items. MaxVisible == 0 means "render all".
	Scroll     int
	MaxVisible int

	FooterHint   string // optional dim line under the list
	EmptyMessage string // shown when items is empty (default: "No items")
}

// OverlayListWidth returns the overlay box width (outer, including border
// and padding) needed to render every row without wrapping. Floored at
// OverlayListFloor, capped at maxWidth when maxWidth > 0. Mirrors the
// floor-and-cap pattern from ActionOverlayWidth.
func OverlayListWidth(items []OverlayListItem, cfg OverlayListConfig, maxWidth int) int {
	hasActive := anyActive(items, cfg)
	contentW := 0
	for _, it := range items {
		if w := lipgloss.Width(itemPlainLine(it, cfg, hasActive)); w > contentW {
			contentW = w
		}
	}
	w := max(contentW+overlayListChrome, OverlayListFloor)
	if maxWidth > 0 && w > maxWidth {
		w = maxWidth
	}
	return w
}

// anyActive reports whether the active-marker column should be reserved.
// The column collapses when ShowActiveMarker is off or when no item is
// currently active, so non-stateful lists (no preset applied, no current
// row marked) render with the bracket flush against the box's left padding.
func anyActive(items []OverlayListItem, cfg OverlayListConfig) bool {
	if !cfg.ShowActiveMarker {
		return false
	}
	for _, it := range items {
		if it.Active {
			return true
		}
	}
	return false
}

// RenderOverlayList renders the list at the given inner content width.
// `innerW` is the width inside OverlayStyle's border+padding — callers
// computing the outer overlay width via OverlayListWidth should subtract
// overlayListChrome before passing it here.
func RenderOverlayList(items []OverlayListItem, cfg OverlayListConfig, innerW int) string {
	var b strings.Builder

	if cfg.Title != "" {
		b.WriteString(OverlayTitleStyle.Render(cfg.Title))
		b.WriteString("\n")
	}
	if cfg.Subtitle != "" {
		b.WriteString(OverlayDimStyle.Render(cfg.Subtitle))
		b.WriteString("\n")
	}
	if cfg.FilterActive || cfg.Filter != "" {
		b.WriteString(OverlayDimStyle.Render("filter: "))
		b.WriteString(OverlayInputStyle.Render(cfg.Filter))
		if cfg.FilterActive {
			b.WriteString(OverlayDimStyle.Render("█"))
		}
		b.WriteString("\n")
	}

	if len(items) == 0 {
		msg := cfg.EmptyMessage
		if msg == "" {
			msg = "No items"
		}
		b.WriteString(OverlayDimStyle.Render(msg))
		if cfg.FooterHint != "" {
			b.WriteString("\n\n")
			b.WriteString(OverlayDimStyle.Render(cfg.FooterHint))
		}
		return b.String()
	}

	// Scroll window.
	hasActive := anyActive(items, cfg)
	start, end := scrollWindow(len(items), cfg.Scroll, cfg.MaxVisible)
	for i := start; i < end; i++ {
		it := items[i]
		if i == cfg.Cursor {
			// Cursor row: plain text padded to innerW so the selection
			// background spans the entire row. Embedded styles would punch
			// holes in the highlight.
			b.WriteString(OverlaySelectedStyle.Width(innerW).Render(itemPlainLine(it, cfg, hasActive)))
		} else {
			b.WriteString(OverlayNormalStyle.Render(itemStyledLine(it, cfg, hasActive)))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	if cfg.FooterHint != "" {
		b.WriteString("\n\n")
		b.WriteString(OverlayDimStyle.Render(cfg.FooterHint))
	}

	return b.String()
}

// itemPlainLine returns the item rendered as plain text (no styling).
// Used for the cursor row (where embedded styles would punch holes in
// the highlight background) and for width measurement. `hasActive`
// controls whether the active-marker column is reserved — see
// anyActive for the collapse rule.
func itemPlainLine(it OverlayListItem, cfg OverlayListConfig, hasActive bool) string {
	var p strings.Builder
	if cfg.MultiSelect {
		if it.Selected {
			p.WriteString("☑ ")
		} else {
			p.WriteString("☐ ")
		}
	}
	if hasActive {
		if it.Active {
			p.WriteString("✓ ")
		} else {
			p.WriteString("  ")
		}
	}
	if cfg.ShowStatus && it.Status != "" {
		fmt.Fprintf(&p, "[%s] ", it.Status)
	}
	if cfg.ShowKey && it.Key != "" {
		fmt.Fprintf(&p, "[%s] ", it.Key)
	}
	p.WriteString(it.Name)
	if cfg.ShowDescription && it.Description != "" {
		p.WriteString("  ")
		p.WriteString(it.Description)
	}
	return p.String()
}

// itemStyledLine returns the item with per-segment styling (dim
// description, filter-color key hint, etc.) suitable for non-cursor rows.
func itemStyledLine(it OverlayListItem, cfg OverlayListConfig, hasActive bool) string {
	var p strings.Builder
	if cfg.MultiSelect {
		if it.Selected {
			p.WriteString(OverlayFilterStyle.Render("☑ "))
		} else {
			p.WriteString("☐ ")
		}
	}
	if hasActive {
		if it.Active {
			p.WriteString(OverlayFilterStyle.Render("✓ "))
		} else {
			p.WriteString("  ")
		}
	}
	if cfg.ShowStatus && it.Status != "" {
		p.WriteString(OverlayFilterStyle.Render(fmt.Sprintf("[%s]", it.Status)))
		p.WriteString(" ")
	}
	if cfg.ShowKey && it.Key != "" {
		p.WriteString(OverlayFilterStyle.Render(fmt.Sprintf("[%s]", it.Key)))
		p.WriteString(" ")
	}
	if it.Disabled {
		p.WriteString(OverlayDimStyle.Render(it.Name))
	} else {
		p.WriteString(it.Name)
	}
	if cfg.ShowDescription && it.Description != "" {
		p.WriteString("  ")
		p.WriteString(OverlayDimStyle.Render(it.Description))
	}
	return p.String()
}

// scrollWindow returns the half-open [start, end) item range to render
// given total item count, current scroll offset, and the maximum number
// of items the overlay can show. MaxVisible == 0 means unbounded.
// Scroll is clamped to a valid range so callers can pass uncorrected
// values without producing out-of-bounds reads.
func scrollWindow(total, scroll, maxVisible int) (int, int) {
	if maxVisible <= 0 || maxVisible >= total {
		return 0, total
	}
	if scroll < 0 {
		scroll = 0
	}
	if maxScroll := total - maxVisible; scroll > maxScroll {
		scroll = maxScroll
	}
	return scroll, scroll + maxVisible
}
