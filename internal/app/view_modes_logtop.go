package app

import (
	"fmt"
	"strings"

	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) viewLogTop() string {
	total := len(m.logTop.parsed)
	rows := make([]ui.LogTopRow, len(m.logTop.rows))
	for i, r := range m.logTop.rows {
		pct := 0.0
		if total > 0 {
			pct = 100 * float64(r.Count) / float64(total)
		}
		rows[i] = ui.LogTopRow{
			Key:      strings.Join(r.Values, " "),
			Count:    r.Count,
			ErrCount: r.ErrCount,
			Pct:      pct,
		}
	}

	prof := string(m.logTop.profile)
	if m.logTop.autoProf {
		prof += " (auto)"
	}
	span := ""
	if m.logTop.lastTS > m.logTop.firstTS {
		span = fmt.Sprintf("  span %.0fs", float64(m.logTop.lastTS-m.logTop.firstTS)/1e9)
	}
	title := fmt.Sprintf("%s    %s   %d matched / %d unmatched%s",
		m.logTop.title, prof, total, m.logTop.unmatched, span)

	hint := m.logTopHintBar()
	return ui.RenderLogTopView(title, m.logTop.groupBy, rows, m.logTopReqPerSec(),
		total, m.logTop.cursor, m.logTop.scroll, hint, m.width, m.height)
}

func (m Model) logTopHintBar() string {
	kb := ui.ActiveKeybindings
	hints := []ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: kb.SortNext, Desc: "sort"},
		{Key: "g", Desc: "group by"},
		{Key: "p", Desc: "profile"},
		{Key: "enter", Desc: "drill"},
		{Key: "esc", Desc: "back"},
	}
	return ui.RenderHintBar(hints, m.width)
}
