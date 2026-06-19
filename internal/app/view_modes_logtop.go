package app

import (
	"fmt"
	"strings"

	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) viewLogTop() string {
	total := len(m.logTop.parsed)
	dims := m.logTopVisibleDims()
	metrics := m.logTopVisibleMetrics()
	rows := make([]ui.LogTopRow, len(m.logTop.rows))
	for i, r := range m.logTop.rows {
		pct := 0.0
		if total > 0 {
			pct = 100 * float64(r.Count) / float64(total)
		}
		rows[i] = ui.LogTopRow{
			Dims:      r.Dims,
			Count:     r.Count,
			ErrCount:  r.ErrCount,
			Pct:       pct,
			P50:       r.P50,
			P95:       r.P95,
			P99:       r.P99,
			Avg:       r.Avg,
			Max:       r.Max,
			Status4xx: r.Status4xx,
			Status5xx: r.Status5xx,
		}
	}

	ui.ActiveSortColumnName = m.logTop.sortCol
	ui.ActiveSortAscending = m.logTop.sortAsc
	ui.ActiveLogTopNextDrill = m.logTopNextDrillDim()

	prof := string(m.logTop.profile)
	if m.logTop.autoProf {
		prof += " (auto)"
	}
	span := ""
	if m.logTop.lastTS > m.logTop.firstTS {
		span = fmt.Sprintf("  span %.0fs", float64(m.logTop.lastTS-m.logTop.firstTS)/1e9)
	}
	var drillParts []string
	for _, fr := range m.logTop.drillStack {
		for _, flt := range fr.filters {
			drillParts = append(drillParts, flt.field+"="+flt.value)
		}
	}
	drill := ""
	if len(drillParts) > 0 {
		drill = "  filter: " + strings.Join(drillParts, " ")
	}
	filterSuffix := ""
	if m.logTop.filterActive {
		filterSuffix = "  /" + m.logTop.filterInput.Value + "█"
	} else if m.logTop.filterQuery != "" {
		filterSuffix = "  filter: " + m.logTop.filterQuery
	}
	searchSuffix := ""
	if m.logTop.searchActive {
		searchSuffix = "  /" + m.logTop.searchInput.Value + "█"
	} else if m.logTop.searchQuery != "" {
		searchSuffix = "  search: " + m.logTop.searchQuery
	}
	title := fmt.Sprintf("%s    %s   %d matched / %d unmatched%s%s%s%s",
		m.logTop.title, prof, total, m.logTop.unmatched, span, drill, filterSuffix, searchSuffix)

	hint := m.logTopHintBar()
	return ui.RenderLogTopView(title, dims, metrics, rows, m.logTopReqPerSec(),
		total, m.logTop.cursor, m.logTop.scroll, hint, m.width, m.height)
}

func (m Model) logTopHintBar() string {
	kb := ui.ActiveKeybindings
	enterDesc := "drill"
	if next := m.logTopNextDrillDim(); next != "" {
		enterDesc = "group by " + next
	}
	hints := []ui.HintEntry{
		{Key: "j/k g/G", Desc: "navigate"},
		{Key: kb.SortNext + "/" + kb.SortPrev, Desc: "sort col"},
		{Key: kb.SortFlip, Desc: "flip sort"},
		{Key: kb.SortReset, Desc: "reset sort"},
		{Key: ".", Desc: "group by"},
		{Key: "p", Desc: "profile"},
		{Key: kb.ColumnToggle, Desc: "columns"},
		{Key: kb.Filter, Desc: "filter"},
		{Key: kb.Search, Desc: "search"},
		{Key: kb.NextMatch + "/" + kb.PrevMatch, Desc: "next/prev"},
		{Key: "tab", Desc: "next dim"},
		{Key: "enter", Desc: enterDesc},
		{Key: "esc", Desc: "back"},
	}
	return ui.RenderHintBar(hints, m.width)
}
