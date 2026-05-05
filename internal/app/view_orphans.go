package app

import (
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) renderOrphansOverlay() (string, int, int) {
	visible := m.orphans.visibleItems()
	rows := ui.AsRows(visible)
	counts := m.orphanCounts()
	partial := ""
	if m.orphans.partial != nil {
		partial = m.orphans.partial.Error()
	}
	w := m.orphansOverlayW()
	h := m.orphansOverlayH()
	body := ui.RenderOrphansOverlay(
		rows, counts, int(m.orphans.visibleKind),
		m.orphans.cursor, m.orphans.scroll,
		w, h,
		m.orphans.filter.Value, m.orphans.filterActive,
		m.orphans.loading, partial,
		m.orphans.strict,
	)
	return body, w, h
}

// orphanCounts builds the per-kind count struct used by both the
// renderer (chip labels) and the move handler (chip wrap height).
// Counts respect the current strict-mode filter so chips show what the
// user will actually see when they Tab to that chip.
func (m Model) orphanCounts() ui.OrphanCounts {
	return ui.OrphanCounts{
		Pods:            m.orphans.orphanKindCount(orphanKindPod),
		Secrets:         m.orphans.orphanKindCount(orphanKindSecret),
		ConfigMaps:      m.orphans.orphanKindCount(orphanKindConfigMap),
		Services:        m.orphans.orphanKindCount(orphanKindService),
		PVCs:            m.orphans.orphanKindCount(orphanKindPVC),
		HPAs:            m.orphans.orphanKindCount(orphanKindHPA),
		PDBs:            m.orphans.orphanKindCount(orphanKindPDB),
		NetworkPolicies: m.orphans.orphanKindCount(orphanKindNetworkPolicy),
		Roles:           m.orphans.orphanKindCount(orphanKindRoles),
		Bindings:        m.orphans.orphanKindCount(orphanKindBindings),
	}
}
