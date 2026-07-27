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

// overlayListChrome is the horizontal chrome that needs to be added to
// the longest content row to compute the outer overlay width passed to
// OverlayStyle.Width(). lipgloss treats Width() as the content area
// INSIDE padding (the border lives outside and is rendered around the
// styled output without counting toward Width), so the chrome that
// affects width math is just the 2+2 cell padding.
const overlayListChrome = 4

// OverlayListItem is a single row in the unified overlay list. Optional
// fields render only when the matching feature flag in OverlayListConfig
// is enabled — callers leave them zero-valued otherwise.
type OverlayListItem struct {
	Key          string // shortcut letter; rendered "[k]" when cfg.ShowKey
	Name         string // primary text
	Description  string // dim secondary suffix; rendered when cfg.ShowDescription
	Status       string // status badge; rendered "[s]" when cfg.ShowStatus
	Active       bool   // ✓ marker when cfg.ShowActiveMarker
	ActiveMarker string // overrides the default ✓ glyph when Active and non-empty
	Selected     bool   // checkbox state when cfg.MultiSelect
	Disabled     bool   // renders dim and (by convention) can't be acted on

	// Header rows render as group dividers ("── Name ──") in CategoryStyle.
	// All other field flags are ignored — no marker, no key, no status,
	// no description. The caller is responsible for keeping cfg.Cursor on
	// a non-Header row (header rows aren't navigable targets).
	Header bool

	// Badge is an optional pre-styled string rendered in a fixed-width
	// column on the right of the row, OUTSIDE the cursor highlight so
	// any styling baked into the string (e.g. a coloured background
	// swatch) stays visible on the selected row. The caller controls
	// the rendering and width — see OverlayListConfig.BadgeWidth.
	Badge string
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

	// Filterable, when true, ALWAYS renders the filter prompt — showing a
	// dim "/ to filter" placeholder when Filter is empty and FilterActive
	// is false. Use this on overlays that accept "/" to enter filter
	// mode: the row sits in the same spot whether or not the user is
	// filtering, so pressing "/" doesn't push the list down by one row.
	Filterable bool

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

	// BadgeWidth, when > 0, reserves this many cells on the right of
	// each row for OverlayListItem.Badge. The badge sits OUTSIDE the
	// cursor highlight so a coloured background (e.g. ClusterColor's
	// swatch) stays visible on the selected row. Items render in
	// innerW - BadgeWidth - 1 cells (the -1 is the space between the
	// row content and the badge).
	BadgeWidth int

	// CursorHighlightWidth, when > 0, caps the selection highlight on the
	// cursor row to this many cells instead of spanning the full item
	// area. The remaining cells up to the item width pad with the surface
	// background so trailing columns (badge/scrollbar) stay aligned. Use
	// it when a full-width bar would read as highlighting trailing fields
	// (e.g. the AutoSync overlay's ON/OFF switches).
	CursorHighlightWidth int

	// Height, when > 0, locks the rendered output to exactly this many
	// lines. Padding with blank lines when content is shorter,
	// truncating when longer. Use this to keep the surrounding overlay
	// box from visibly resizing when the filter row appears / disappears
	// or items are filtered out. Pass `overlayH - 2` to account for the
	// 1+1 vertical padding lipgloss adds inside OverlayStyle.
	Height int
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
	return OverlayContentWidth(contentW, maxWidth)
}

// OverlayContentWidth returns the overlay box width that shows contentW
// columns of content, applying the same chrome, floor and cap as
// OverlayListWidth. Overlays that render their own content below the list
// (the taint editor's add-row) size themselves against this so the box grows
// to fit them instead of truncating them against a list-derived width.
func OverlayContentWidth(contentW, maxWidth int) int {
	w := max(contentW+overlayListChrome, OverlayListFloor)
	if maxWidth > 0 && w > maxWidth {
		w = maxWidth
	}
	return w
}

// anyActive reports whether the active-marker column should be reserved.
// Returns cfg.ShowActiveMarker so the column is ALWAYS present when the
// caller asks for it — collapsing the column when no item happens to be
// active produced a visible shift in lists whose active state changes at
// runtime (namespace selector items hopping left/right as the user
// filtered and selected). The 2-cell placeholder is a small cosmetic
// cost in exchange for stable row alignment.
func anyActive(_ []OverlayListItem, cfg OverlayListConfig) bool {
	return cfg.ShowActiveMarker
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
	if cfg.Filterable || cfg.FilterActive || cfg.Filter != "" {
		switch {
		case cfg.FilterActive:
			b.WriteString(OverlayFilterStyle.Render("/ " + cfg.Filter))
			b.WriteString(OverlayDimStyle.Render("█"))
		case cfg.Filter != "":
			b.WriteString(OverlayFilterStyle.Render("/ " + cfg.Filter))
		default:
			b.WriteString(OverlayDimStyle.Render("/ to filter"))
		}
		b.WriteString("\n\n")
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
		out := b.String()
		if cfg.Height > 0 {
			out = PadToHeight(out, cfg.Height)
		}
		return out
	}

	// Scroll window.
	hasActive := anyActive(items, cfg)
	start, end := scrollWindow(len(items), cfg.Scroll, cfg.MaxVisible)
	visible := end - start
	hasOverflow := len(items) > visible
	// Reserve trailing columns for the badge (when any items carry one)
	// and the scrollbar (when the list overflows). Both sit OUTSIDE the
	// cursor highlight so their styling/visibility stays intact on the
	// selected row.
	badgeReserve := 0
	if cfg.BadgeWidth > 0 {
		badgeReserve = cfg.BadgeWidth + 1 // +1 for the space separator
	}
	scrollReserve := 0
	if hasOverflow {
		scrollReserve = 1
	}
	itemWidth := max(innerW-badgeReserve-scrollReserve, 1)
	for i := start; i < end; i++ {
		it := items[i]
		var row string
		if i == cfg.Cursor {
			// Cursor row: plain text padded so the selection background spans
			// the item area. The scrollbar + badge sit outside the highlight
			// so they stay readable against the box. CursorHighlightWidth
			// caps the highlight short of the item area; the remainder pads
			// with the surface background so the badge column stays put.
			// Text is truncated to the highlight width — anything longer
			// would wrap inside the Width(hlW) block, pushing every row
			// below out of sync with the scrollbar column.
			hlW := itemWidth
			if cfg.CursorHighlightWidth > 0 && cfg.CursorHighlightWidth < itemWidth {
				hlW = cfg.CursorHighlightWidth
			}
			row = OverlaySelectedStyle.Width(hlW).Render(Truncate(itemPlainLine(it, cfg, hasActive), hlW))
			if pad := itemWidth - lipgloss.Width(row); pad > 0 {
				row += OverlayDimStyle.Render(strings.Repeat(" ", pad))
			}
		} else {
			// Truncated to the item area for the same reason as the cursor
			// row: the component owns the no-wider-than-innerW invariant;
			// callers only best-effort their Name/Description budgets.
			line := OverlayNormalStyle.Render(Truncate(itemStyledLine(it, cfg, hasActive), itemWidth))
			// Only pad non-cursor rows when there's a trailing column
			// (badge or scrollbar) that needs a stable left edge —
			// padding rows without a reserve regresses the historical
			// "non-cursor rows keep their short visible width" behaviour.
			// The padding renders through OverlayDimStyle so it carries the
			// theme's surface background — raw spaces would punch through to
			// the terminal background between the row text and the badge /
			// scrollbar column.
			if hasOverflow || cfg.BadgeWidth > 0 {
				if pad := itemWidth - lipgloss.Width(line); pad > 0 {
					line += OverlayDimStyle.Render(strings.Repeat(" ", pad))
				}
			}
			row = line
		}
		if cfg.BadgeWidth > 0 {
			row += OverlayDimStyle.Render(" ") + it.Badge
		}
		if hasOverflow {
			row += renderScrollbar(i-start, visible, len(items), start)
		}
		b.WriteString(row)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	if cfg.FooterHint != "" {
		b.WriteString("\n\n")
		b.WriteString(OverlayDimStyle.Render(cfg.FooterHint))
	}

	out := b.String()
	if cfg.Height > 0 {
		out = PadToHeight(out, cfg.Height)
	}
	return out
}

// itemPlainLine returns the item rendered as plain text (no styling).
// Used for the cursor row (where embedded styles would punch holes in
// the highlight background) and for width measurement. `hasActive`
// controls whether the active-marker column is reserved — see
// anyActive for the collapse rule.
// activeMarkerGlyph returns the single-cell marker drawn for an Active row,
// defaulting to ✓ unless the item overrides it (e.g. "!" for excluded items).
func activeMarkerGlyph(it OverlayListItem) string {
	if it.ActiveMarker != "" {
		return it.ActiveMarker
	}
	return "✓"
}

func itemPlainLine(it OverlayListItem, cfg OverlayListConfig, hasActive bool) string {
	if it.Header {
		return "── " + it.Name + " ──"
	}
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
			p.WriteString(activeMarkerGlyph(it) + " ")
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
	if it.Header {
		return CategoryStyle.Render("── " + it.Name + " ──")
	}
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
			p.WriteString(OverlayFilterStyle.Render(activeMarkerGlyph(it) + " "))
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

// renderScrollbar returns the single-cell scrollbar character to draw on
// the right of the row at visualIdx (0-indexed within the visible window).
// Thumb size is proportional to viewport coverage (visible/total); thumb
// position interpolates linearly from 0 to (visible - thumb) across the
// scroll range. Track uses OverlayDimStyle; thumb uses OverlayFilterStyle
// so it reads against the dim track without competing with the cursor
// highlight or filter-row colour.
func renderScrollbar(visualIdx, totalVisible, totalItems, startIdx int) string {
	if totalItems <= totalVisible {
		return " "
	}
	thumbSize := min(max(totalVisible*totalVisible/totalItems, 1), totalVisible)
	maxScroll := totalItems - totalVisible
	maxThumbTop := totalVisible - thumbSize
	thumbTop := 0
	if maxScroll > 0 {
		thumbTop = (maxThumbTop * startIdx) / maxScroll
	}
	if visualIdx >= thumbTop && visualIdx < thumbTop+thumbSize {
		return OverlayFilterStyle.Render("█")
	}
	return OverlayDimStyle.Render("│")
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
