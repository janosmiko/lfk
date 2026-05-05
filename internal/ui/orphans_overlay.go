package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/k8s"
)

// OrphanRow is the renderer's input — minimum data to render one table
// row. Field names match OrphanItem so the caller is a trivial copy.
type OrphanRow struct {
	Kind, Namespace, Name, Reason string
}

// OrphanCounts holds the kind-bucketed counts displayed in the chips
// header. Defined as a named type so the call site is self-documenting.
type OrphanCounts struct {
	Pods, Secrets, ConfigMaps, Services, PVCs int
	HPAs, PDBs, NetworkPolicies               int
	Roles, Bindings                           int
}

// OrphanScrollForCursor returns the new scroll offset that keeps
// `cursor` visible inside a viewport of `bodyHeight` rows starting at
// `scroll`. Vim semantics: do nothing if the cursor is already in
// view; otherwise scroll just enough to put the cursor on the nearest
// visible edge. Mirrors WhoCanScrollForCursor — keep them in sync.
func OrphanScrollForCursor(scroll, cursor, bodyHeight, total int) int {
	if total <= bodyHeight {
		return 0
	}
	if cursor < scroll {
		return cursor
	}
	if cursor >= scroll+bodyHeight {
		return cursor - bodyHeight + 1
	}
	return scroll
}

// OrphanClampScroll snaps the requested scroll offset into the valid
// range so the rendered slice never extends past the end of the list.
// Handlers should call this after any operation that shrinks the list
// (kind switch, filter typed) so a stale offset doesn't render past
// len(rows).
func OrphanClampScroll(scroll, total, bodyHeight int) int {
	if total <= bodyHeight {
		return 0
	}
	maxScroll := total - bodyHeight
	scroll = max(scroll, 0)
	scroll = min(scroll, maxScroll)
	return scroll
}

// OrphanBodyHeight returns the number of body rows that fit inside an
// overlay of outer height `overlayH`, accounting for OverlayStyle's
// padding (-2) and the renderer's chrome rows. Both flags must reflect
// what the renderer ACTUALLY emits or the cursor lands on a row that
// PadToHeight then clips — which the user sees as "j moves cursor off
// the bottom and nothing scrolls". Caller (the app-side adapter)
// computes the same value for the move handlers — they MUST agree.
//
// Chrome lines (after split by "\n"):
//
//	[0]   "Orphans (cluster-wide) — strict"   title text
//	[1]   "                      "            title's invisible bottom-padding row
//	[2]   ""                                  blank from "\n\n"
//	[3..] chips strip (1+ rows — see chipLines)
//	[N]   (banner)                            only if hasPartial
//	[N+1] (search)                            only if hasSearch
//	[N+2] ""                                  blank-before-body
//	[N+3] header
//	[N+4] ""                                  trailing empty from header's "\n"
//
// chipLines must equal the wrapped row count emitted by
// renderOrphanChips for the same width and counts; callers compute it
// via OrphanChipLines so the renderer and the move handler always
// agree on viewport size. With all 11 kinds always shown, the strip
// wraps to 2 rows on a typical 100-col overlay and to 3+ rows on
// narrower terminals — passing the wrong value pushes the cursor off
// the visible viewport.
//
// → 6 fixed-chrome rows + chipLines + 1 per optional row.
func OrphanBodyHeight(overlayH int, chipLines int, hasPartial, hasSearch bool) int {
	innerH := overlayH - 2 // OverlayStyle.Padding(1, 2) → -1 top, -1 bottom
	if chipLines < 1 {
		chipLines = 1
	}
	chrome := 6 + chipLines
	if hasPartial {
		chrome++
	}
	if hasSearch {
		chrome++
	}
	return max(innerH-chrome, 1)
}

// OrphanChipLines returns the number of wrapped rows renderOrphanChips
// will produce for the given counts and inner width. The move handler
// calls this so its viewport math matches the renderer's exactly —
// without it, the worst-case fallback would either reserve too few
// lines (cursor lands off-screen) or too many (waste body rows).
func OrphanChipLines(counts OrphanCounts, width int) int {
	_, cols := orphanChipLayout(counts, width)
	return (len(orphanChipDefs(counts)) + cols - 1) / cols
}

// RenderOrphansOverlay produces the cluster-wide orphan overlay content,
// padded to exactly `height-2` lines so the OverlayStyle frame stays a
// fixed size regardless of how many rows are in the report. Caller wraps
// the result in `OverlayStyle.Width(width).Height(height).Render(...)`.
//
// `activeChip` is 0=All, 1=Pod, 2=Secret, 3=ConfigMap, 4=Service.
// `scroll` is the first visible row index — caller maintains it via
// OrphanScrollForCursor so the cursor stays in view as it moves.
func RenderOrphansOverlay(
	rows []OrphanRow,
	counts OrphanCounts,
	activeChip int,
	cursor, scroll int,
	width, height int,
	searchQuery string,
	searchActive bool,
	loading bool,
	partialError string,
	strict bool,
) string {
	innerH := max(height-2, 1)

	var b strings.Builder
	title := "Orphans (cluster-wide)"
	if strict {
		title += " — strict"
	} else {
		title += " — lenient (incl. workload-template refs)"
	}
	b.WriteString(OverlayTitleStyle.Render(title))
	b.WriteString("\n\n")

	// width-4 = OverlayStyle.Padding(1, 2) horizontal cost; the chip row
	// must fit inside the inner content area.
	chips := renderOrphanChips(counts, activeChip, max(width-4, 40))
	chipLines := strings.Count(chips, "\n") + 1
	b.WriteString(chips)
	b.WriteString("\n")

	hasPartial := partialError != ""
	hasSearch := searchActive || searchQuery != ""
	if hasPartial {
		b.WriteString(OverlayWarningStyle.Render("partial result: " + partialError))
		b.WriteString("\n")
	}
	if hasSearch {
		b.WriteString(OverlayNormalStyle.Render("  / "))
		b.WriteString(OverlayInputStyle.Render(searchQuery))
		if searchActive {
			b.WriteString(OverlayDimStyle.Render("█"))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if loading {
		b.WriteString(OverlayDimStyle.Render("  Scanning cluster…"))
		return PadToHeight(b.String(), innerH)
	}
	if len(rows) == 0 {
		b.WriteString(OverlayDimStyle.Render("  No orphans found"))
		return PadToHeight(b.String(), innerH)
	}

	// Column widths — derived from inner overlay width so narrow
	// terminals don't wrap. Keep KIND and REASON tight (their values are
	// known short strings) and split the slack between NAMESPACE and
	// NAME, where the long values actually live.
	innerW := max(width-4, 40) // OverlayStyle.Padding(1, 2) → -4 horizontal
	kindW := 11                // longest is "ConfigMap" (9)
	reasonW := 22              // longest is "no owner (terminal)" (19)
	nsW := min(20, max(8, innerW/4))
	nameW := max(10, innerW-(2+kindW+1+nsW+1+reasonW))

	// Header row.
	b.WriteString(OverlayDimStyle.Render(fmt.Sprintf("  %-*s %-*s %-*s %s",
		kindW, Truncate("KIND", kindW),
		nsW, Truncate("NAMESPACE", nsW),
		nameW, Truncate("NAME", nameW),
		Truncate("REASON", reasonW),
	)))
	b.WriteString("\n")

	bodyHeight := OrphanBodyHeight(height, chipLines, hasPartial, hasSearch)
	scroll = OrphanClampScroll(scroll, len(rows), bodyHeight)
	end := min(scroll+bodyHeight, len(rows))
	for i := scroll; i < end; i++ {
		r := rows[i]
		line := fmt.Sprintf("  %-*s %-*s %-*s %s",
			kindW, Truncate(r.Kind, kindW),
			nsW, Truncate(r.Namespace, nsW),
			nameW, Truncate(r.Name, nameW),
			Truncate(r.Reason, reasonW),
		)
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else {
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	return PadToHeight(b.String(), innerH)
}

// orphanChipDef is one entry in the chip strip — the label, the
// current count, and the kind index used by the model's Tab cycle.
type orphanChipDef struct {
	label string
	count int
	idx   int
}

// orphanChipDefs returns every kind chip in display order. All kinds
// are always present so the strip is a stable map of cluster orphan
// state (Tab-cycling lands on a known position) rather than a list
// that shrinks/grows as counts cross zero.
func orphanChipDefs(counts OrphanCounts) []orphanChipDef {
	total := counts.Pods + counts.Secrets + counts.ConfigMaps + counts.Services + counts.PVCs +
		counts.HPAs + counts.PDBs + counts.NetworkPolicies + counts.Roles + counts.Bindings
	return []orphanChipDef{
		{"All", total, 0},
		{"Pods", counts.Pods, 1},
		{"Secrets", counts.Secrets, 2},
		{"CMs", counts.ConfigMaps, 3},
		{"Svcs", counts.Services, 4},
		{"PVCs", counts.PVCs, 5},
		{"HPAs", counts.HPAs, 6},
		{"PDBs", counts.PDBs, 7},
		{"NetPols", counts.NetworkPolicies, 8},
		{"Roles", counts.Roles, 9},
		{"RBs", counts.Bindings, 10},
	}
}

// orphanChipLayout returns the widest unpadded chip text and the
// column count for the chip grid given counts and inner width.
// Renderer and move handler share this so their wrap row counts always
// match exactly; the renderer also reuses maxTextW so every cell pads
// to a uniform width.
func orphanChipLayout(counts OrphanCounts, width int) (maxTextW, cols int) {
	defs := orphanChipDefs(counts)
	for _, d := range defs {
		w := lipgloss.Width(fmt.Sprintf("%s %d", d.label, d.count))
		if w > maxTextW {
			maxTextW = w
		}
	}
	const gutter = "  "
	const indent = "  "
	cellW := maxTextW + 2 + lipgloss.Width(gutter)
	avail := max(width-lipgloss.Width(indent), cellW)
	cols = max(avail/cellW, 1)
	return maxTextW, cols
}

// renderOrphanChips builds the header chip strip in the same visual
// style the CrashLoopBackOff investigator uses for its tab bar
// (renderCrashTabBar): every chip text is wrapped in single-space
// padding, the active chip uses OverlaySelectedStyle (bg fills the
// whole padded run, making it read as a button), inactive chips use
// OverlayDimStyle (dim fg, padding for symmetry, no bg). Chips are
// joined with a 2-space gutter — no middle-dot separator, since the
// active chip's bg already provides the visual seam.
//
// Every kind is always shown — including zero-count chips — so the
// strip is a complete map of cluster orphan state. This lets users
// see at a glance "no PVCs are orphaned" without having to remember
// whether their absence means "no orphans" or "the chip is hidden".
// Cells are padded to a uniform width so column N of row 1 lines up
// with column N of row 2 — with 11 chips the strip wraps to 2 rows on
// a typical 100-col overlay and to 3+ rows on narrower terminals.
//
// Result reads like the crash-investigator tab bar but tabular when
// it has to wrap:
//
//	[ All 6 ]   Pods 3      Secrets 2   CMs 1
//	Svcs 0      PVCs 1      HPAs 0      PDBs 0
//	NetPols 0   Roles 0     RBs 0
//
// (where `[ All 6 ]` denotes the active chip with bg highlight).
func renderOrphanChips(counts OrphanCounts, active, width int) string {
	defs := orphanChipDefs(counts)
	maxTextW, cols := orphanChipLayout(counts, width)

	cells := make([]string, 0, len(defs))
	for _, d := range defs {
		// Pad the inner text to maxTextW so every chip occupies the
		// same visual width. Surround with single-space padding (matches
		// renderCrashTabBar). The styled span covers all of it — for
		// the active chip the bg fills the whole padded button; for
		// inactive chips the dim fg covers the padding too, and bg is
		// the surrounding overlay surface.
		inner := fmt.Sprintf(" %-*s ", maxTextW, fmt.Sprintf("%s %d", d.label, d.count))
		if d.idx == active {
			cells = append(cells, OverlaySelectedStyle.Render(inner))
		} else {
			cells = append(cells, OverlayDimStyle.Render(inner))
		}
	}

	// 2-space gutter between chips (matches renderCrashTabBar). The
	// active chip's bg fill provides the visible seam between cells —
	// no need for a middle-dot separator competing with it.
	const gutter = "  "
	const indent = "  "

	rows := make([]string, 0, (len(cells)+cols-1)/cols)
	for i := 0; i < len(cells); i += cols {
		end := min(i+cols, len(cells))
		var b strings.Builder
		b.WriteString(indent)
		for j := i; j < end; j++ {
			if j > i {
				b.WriteString(gutter)
			}
			b.WriteString(cells[j])
		}
		// Pad short rows out to the full grid width so every wrapped
		// row reports the same lipgloss.Width — without that, when
		// the last row has fewer chips than `cols` the alignment test
		// (and the user's eye) sees a ragged trailing edge.
		if end-i < cols {
			missing := cols - (end - i)
			cellWtext := maxTextW + 2 // unstyled width of one cell's content
			for range missing {
				b.WriteString(gutter)
				b.WriteString(strings.Repeat(" ", cellWtext))
			}
		}
		rows = append(rows, b.String())
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// AsRows converts OrphanItem values into the renderer's row type so the
// app package can call this without coupling ui to k8s types at the call site.
func AsRows(items []k8s.OrphanItem) []OrphanRow {
	out := make([]OrphanRow, len(items))
	for i, it := range items {
		out[i] = OrphanRow{Kind: it.Kind, Namespace: it.Namespace, Name: it.Name, Reason: it.Reason}
	}
	return out
}
