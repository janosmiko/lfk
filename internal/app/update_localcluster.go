// update_localcluster routes key and message events for the local-
// cluster manager overlay. The overlay opens via Ctrl+N at
// LevelClusters and stays open across sub-screens until Esc on the
// list view closes it.
package app

import (
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/k8s/localcluster"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// buildLocalClusterOverlayState assembles the renderer input from the
// Model's local-cluster state. Width/Height are filled by the caller
// from the overlay-box dimensions so the renderer can pad to the same
// inner height OverlayStyle declares.
func (m Model) buildLocalClusterOverlayState() ui.LocalClusterOverlayState {
	provs := make([]string, 0, 3)
	for _, p := range localcluster.Installed() {
		provs = append(provs, p.Name())
	}
	// Single source of truth: m.localClusterState.clusters carries
	// every row, distinguished by .state (Real / InFlight / Failed).
	// The renderer paints all three with the same column layout; the
	// Mutating pill is what visually flags InFlight rows.
	rows := make([]ui.LocalClusterRowView, 0, len(m.localClusterState.clusters))
	for _, r := range m.localClusterState.clusters {
		rows = append(rows, ui.LocalClusterRowView{
			Provider:   r.Provider,
			Name:       r.Name,
			Status:     r.Status,
			K8sVersion: r.K8sVersion,
			Nodes:      r.Nodes,
			Age:        r.Age,
			Mutating:   r.Mutating,
			ListError:  r.ListError,
		})
	}
	return ui.LocalClusterOverlayState{
		ProvidersInstalled: provs,
		Clusters:           rows,
		Cursor:             m.localClusterState.cursor,
		Loading:            m.localClusterState.loading,
		Info:               m.localClusterState.info,
		GlobalErr:          m.localClusterState.err,
	}
}

// openLocalClusterManager opens the overlay and kicks off Detect.
func (m Model) openLocalClusterManager() (Model, tea.Cmd) {
	m.overlay = overlayLocalClusters
	gen := m.localClusterState.gen + 1
	m.localClusterState = localClusterState{
		screen:  localClusterScreenList,
		gen:     gen,
		loading: true,
	}
	return m, m.dispatchDetectLocalClusters(gen, localcluster.All())
}

// updateLocalClustersDetected applies the Detect result if its gen
// token matches the current state. Stale results from a superseded
// fetch are dropped silently.
//
//nolint:unparam // consistent message handler signature; tea.Cmd return is consumed by central dispatch.
func (m Model) updateLocalClustersDetected(msg localClustersDetectedMsg) (Model, tea.Cmd) {
	if msg.gen != m.localClusterState.gen {
		return m, nil
	}
	rows := make([]localClusterRow, 0, len(msg.clusters))
	for _, c := range msg.clusters {
		rows = append(rows, localClusterRow{
			Provider:    c.Provider,
			Name:        c.Name,
			ContextName: c.ContextName,
			Status:      string(c.Status),
			K8sVersion:  c.K8sVersion,
			Nodes:       c.Nodes,
			Age:         c.Age,
			state:       rowStateReal,
		})
	}
	// Apply per-provider List errors:
	//   - row-level (ListError) when at least one cluster from that provider made it through
	//   - global header err otherwise (provider with no clusters returned an error)
	if len(msg.providerErrors) > 0 {
		rowsWithProvider := map[string]bool{}
		for _, r := range rows {
			rowsWithProvider[r.Provider] = true
		}
		var globalParts []string
		for prov, errStr := range msg.providerErrors {
			if rowsWithProvider[prov] {
				for i := range rows {
					if rows[i].Provider == prov {
						rows[i].ListError = errStr
					}
				}
			} else {
				globalParts = append(globalParts, prov+": "+errStr)
			}
		}
		sort.Strings(globalParts)
		m.localClusterState.err = strings.Join(globalParts, "; ")
	} else {
		m.localClusterState.err = ""
	}
	// Preserve synthetic rows (InFlight + Failed) whose real cluster
	// hasn't shown up. Without this, Detect would blow away the
	// synthetic "creating..." row from the wizard, AND a Failed row
	// would silently disappear instead of staying visible until the
	// user explicitly retries or deletes. The provider/name key avoids
	// duplicates if the real cluster does materialize.
	realPair := map[string]bool{}
	for _, r := range rows {
		realPair[r.Provider+"/"+r.Name] = true
	}
	for _, c := range m.localClusterState.clusters {
		if c.state == rowStateReal {
			continue
		}
		if !realPair[c.Provider+"/"+c.Name] {
			rows = append(rows, c)
		}
	}
	m.localClusterState.clusters = rows
	m.localClusterState.loading = false
	if m.localClusterState.cursor >= len(rows) {
		m.localClusterState.cursor = 0
	}
	// Refresh the cache from Real rows only — InFlight rows aren't
	// canonical state worth persisting.
	realRows := rows[:0:0]
	for _, r := range rows {
		if r.state == rowStateReal {
			realRows = append(realRows, r)
		}
	}
	m.refreshLocalClusterCache(realRows)
	return m, nil
}

// refreshLocalClusterCache mirrors the manager's view into the on-Model
// cache and persists it. Best-effort save: a write failure is logged
// at the storage layer; the in-memory state stays authoritative.
//
// IMPORTANT receiver semantics: this is a pointer receiver method
// called from updateLocalClustersDetected, which has a value receiver.
// Go auto-addresses the local copy of `m` for the call, so the mutation
// to `m.localClusterCache` lands on that copy — which is then returned
// by value at the end of updateLocalClustersDetected, propagating the
// new map back through the bubbletea Update return. If you ever extract
// this into a non-addressable context (e.g. through an interface), the
// mutation will silently land on the receiver-copy and disappear.
// Either keep the call sites addressable, or convert this to a
// value-receiver method that returns the new map for the caller to
// assign.
func (m *Model) refreshLocalClusterCache(rows []localClusterRow) {
	out := make(map[string]localClusterCacheEntry, len(rows))
	entries := make([]localClusterCacheEntry, 0, len(rows))
	for _, r := range rows {
		e := localClusterCacheEntry{
			Provider:    r.Provider,
			Name:        r.Name,
			ContextName: r.ContextName,
			Status:      r.Status,
			K8sVersion:  r.K8sVersion,
			Nodes:       r.Nodes,
			Age:         r.Age,
			LastSeen:    time.Now(),
		}
		out[r.ContextName] = e
		entries = append(entries, e)
	}
	m.localClusterCache = out
	_ = saveLocalClusterState(entries)
}

// updateLocalClusterCreated handles the result of a wizard-driven
// Create. Clears the synthetic in-flight row (added by
// updateWizardConfirmKey), sets a status banner so the user sees the
// outcome, and re-runs Detect + loadContexts so the new cluster lands
// in both the manager table and the cluster picker.
func (m Model) updateLocalClusterCreated(msg localClusterCreatedMsg) (Model, tea.Cmd) {
	// Drop late results from a superseded session (manager closed or
	// re-opened). loadContexts is the one side-effect that should still
	// fire even on stale: the kubeconfig genuinely changed, so the
	// picker needs to refresh regardless of whether the manager is
	// still open.
	if msg.gen != m.localClusterState.gen {
		if msg.err == nil {
			if reload := m.loadContextsReload(); reload != nil {
				return m, reload
			}
		}
		return m, nil
	}
	// On success, drop the InFlight row matching this provider/name —
	// the real row will land via the Detect we re-fire below. On
	// error, mark the InFlight row Failed so the user sees the row
	// stay (without the Mutating pill) until they explicitly delete
	// or retry.
	for i := range m.localClusterState.clusters {
		r := &m.localClusterState.clusters[i]
		if r.state == rowStateInFlight && r.Provider == msg.provider && r.Name == msg.name {
			if msg.err != nil {
				r.state = rowStateFailed
				r.Mutating = ""
				r.Status = "failed"
			} else {
				// Drop the InFlight row in place; Detect repopulates.
				m.localClusterState.clusters = append(m.localClusterState.clusters[:i], m.localClusterState.clusters[i+1:]...)
			}
			break
		}
	}

	if msg.err != nil {
		m.localClusterState.info = ""
		m.localClusterState.err = "Create " + msg.provider + "/" + msg.name + " failed: " + msg.err.Error()
		// Re-run Detect anyway: a partial create may have left a real
		// row behind (e.g. kind built containers but kubeconfig write
		// failed) and the user needs to see whatever survived to clean
		// up.
		m.localClusterState.gen++
		return m, m.dispatchDetectLocalClusters(m.localClusterState.gen, localcluster.All())
	}
	m.localClusterState.err = ""
	m.localClusterState.info = "Created " + msg.provider + "/" + msg.name
	m.localClusterState.gen++
	cmds := []tea.Cmd{m.dispatchDetectLocalClusters(m.localClusterState.gen, localcluster.All())}
	if reload := m.loadContextsReload(); reload != nil {
		cmds = append(cmds, reload)
	}
	return m, tea.Batch(cmds...)
}

// updateLocalClusterKey is the key dispatcher for the overlay.
// Returns (model, cmd, true) when the key was consumed.
func (m Model) updateLocalClusterKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	if m.overlay != overlayLocalClusters {
		return m, nil, false
	}
	switch m.localClusterState.screen {
	case localClusterScreenList:
		return m.updateLocalClusterListKey(msg)
	case localClusterScreenWizardProvider:
		return m.updateWizardProviderKey(msg)
	case localClusterScreenWizardName:
		return m.updateWizardNameKey(msg)
	case localClusterScreenWizardVersion:
		return m.updateWizardVersionKey(msg)
	case localClusterScreenWizardNodes:
		return m.updateWizardNodesKey(msg)
	case localClusterScreenWizardConfirm:
		return m.updateWizardConfirmKey(msg)
	case localClusterScreenDeleteConfirm:
		return m.updateDeleteConfirmKey(msg)
	}
	// Sub-screens added in later tasks consume keys without effect for now
	// so half-typed wizard input doesn't leak through to the explorer.
	return m, nil, true
}

func (m Model) updateLocalClusterListKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyEsc:
		m.overlay = overlayNone
		return m, nil, true
	case tea.KeyEnter:
		if m.localClusterState.cursor >= len(m.localClusterState.clusters) {
			return m, nil, true
		}
		target := m.localClusterState.clusters[m.localClusterState.cursor].ContextName
		for i, it := range m.middleItems {
			if it.Name == target {
				m.cursors[model.LevelClusters] = i
				break
			}
		}
		m.overlay = overlayNone
		return m, nil, true
	case tea.KeyUp:
		if m.localClusterState.cursor > 0 {
			m.localClusterState.cursor--
		}
		return m, nil, true
	case tea.KeyDown:
		if m.localClusterState.cursor < len(m.localClusterState.clusters)-1 {
			m.localClusterState.cursor++
		}
		return m, nil, true
	}
	switch msg.String() {
	case "j":
		if m.localClusterState.cursor < len(m.localClusterState.clusters)-1 {
			m.localClusterState.cursor++
		}
		return m, nil, true
	case "k":
		if m.localClusterState.cursor > 0 {
			m.localClusterState.cursor--
		}
		return m, nil, true
	case "q":
		m.overlay = overlayNone
		return m, nil, true
	case "R":
		// Refresh BOTH the manager's cluster table (Detect) and the
		// kubeconfig context list (loadContexts). Without the latter
		// the user has to close + reopen the picker after creating a
		// minikube cluster externally.
		m.localClusterState.gen++
		m.localClusterState.loading = true
		cmds := []tea.Cmd{m.dispatchDetectLocalClusters(m.localClusterState.gen, localcluster.All())}
		if reload := m.loadContextsReload(); reload != nil {
			cmds = append(cmds, reload)
		}
		return m, tea.Batch(cmds...), true
	case "n":
		return m.startWizard(), nil, true
	case "s":
		return m.handleListMutate("start")
	case "S":
		return m.handleListMutate("stop")
	case "D":
		if m.localClusterState.cursor >= len(m.localClusterState.clusters) {
			return m, nil, true
		}
		m.localClusterState.deleteRow = m.localClusterState.cursor
		m.localClusterState.deleteBuf = ""
		m.localClusterState.screen = localClusterScreenDeleteConfirm
		return m, nil, true
	}
	return m, nil, true
}

// updateDeleteConfirmKey runs the "type DELETE to confirm" sub-screen.
// Esc cancels back to the list, Backspace edits the buffer, uppercase
// runes append, and Enter only dispatches when the buffer matches the
// literal string "DELETE".
func (m Model) updateDeleteConfirmKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch msg.Type {
	case tea.KeyEsc:
		m.localClusterState.deleteBuf = ""
		m.localClusterState.screen = localClusterScreenList
		return m, nil, true
	case tea.KeyEnter:
		if m.localClusterState.deleteBuf != "DELETE" {
			return m, nil, true
		}
		// Re-check bounds: a background Detect refresh may have shrunk
		// the cluster list while the user was typing DELETE. Without
		// this guard, indexing clusters[deleteRow] could panic on a
		// stale index. buildLocalClusterDeleteConfirmView already does
		// the same defensive check for the renderer side.
		if m.localClusterState.deleteRow < 0 || m.localClusterState.deleteRow >= len(m.localClusterState.clusters) {
			m.localClusterState.err = "delete target no longer in list; refresh and retry"
			m.localClusterState.deleteBuf = ""
			m.localClusterState.screen = localClusterScreenList
			return m, nil, true
		}
		row := &m.localClusterState.clusters[m.localClusterState.deleteRow]
		prov := localcluster.ByName(row.Provider)
		if prov == nil {
			m.localClusterState.err = "unknown provider: " + row.Provider
			m.localClusterState.deleteBuf = ""
			m.localClusterState.screen = localClusterScreenList
			return m, nil, true
		}
		row.Mutating = "deleting..."
		cmd := m.dispatchDeleteLocalCluster(prov, row.Name)
		m.localClusterState.deleteBuf = ""
		m.localClusterState.screen = localClusterScreenList
		return m, cmd, true
	case tea.KeyBackspace:
		if n := len(m.localClusterState.deleteBuf); n > 0 {
			m.localClusterState.deleteBuf = m.localClusterState.deleteBuf[:n-1]
		}
		return m, nil, true
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			if r >= 'A' && r <= 'Z' {
				m.localClusterState.deleteBuf += string(r)
			}
		}
		return m, nil, true
	}
	return m, nil, true
}

// handleListMutate is shared by s (stop) and S (start). It validates
// that the highlighted row supports the op, sets the mutating pill,
// and returns the start/stop cmd.
func (m Model) handleListMutate(verb string) (Model, tea.Cmd, bool) {
	if m.localClusterState.cursor >= len(m.localClusterState.clusters) {
		return m, nil, true
	}
	row := &m.localClusterState.clusters[m.localClusterState.cursor]
	if row.Mutating != "" {
		return m, nil, true
	}
	// Type-assert to LifecycleProvider — kind doesn't satisfy it (no
	// native start/stop), so the s/S keypress falls through silently
	// for kind rows. The hint bar still advertises the keys; the cost
	// of greying them per-row isn't worth the renderer complexity.
	lp, ok := localcluster.ByName(row.Provider).(localcluster.LifecycleProvider)
	if !ok {
		return m, nil, true
	}
	pill := "stopping..."
	cmd := m.dispatchStopLocalCluster(lp, row.Name)
	if verb == "start" {
		pill = "starting..."
		cmd = m.dispatchStartLocalCluster(lp, row.Name)
	}
	row.Mutating = pill
	return m, cmd, true
}

// updateLocalClusterMutated handles a Stop/Start/Delete result. The
// per-row Mutating pill is cleared and Detect is re-run to refresh
// status. On a delete, the contexts list is also reloaded so the
// kubeconfig context disappears from the cluster picker.
func (m Model) updateLocalClusterMutated(msg localClusterMutatedMsg) (Model, tea.Cmd) {
	// Drop late results from a superseded session. Same loadContexts
	// exception as updateLocalClusterCreated for "deleted" — the
	// kubeconfig context is genuinely gone, so the picker needs to
	// refresh regardless of whether the manager is still open.
	if msg.gen != m.localClusterState.gen {
		if msg.verb == "deleted" && msg.err == nil {
			if reload := m.loadContextsReload(); reload != nil {
				return m, reload
			}
		}
		return m, nil
	}
	for i := range m.localClusterState.clusters {
		r := &m.localClusterState.clusters[i]
		if r.Provider == msg.provider && r.Name == msg.name {
			r.Mutating = ""
			if msg.err != nil {
				m.localClusterState.err = msg.err.Error()
			}
			break
		}
	}
	m.localClusterState.gen++
	cmds := []tea.Cmd{m.dispatchDetectLocalClusters(m.localClusterState.gen, localcluster.All())}
	if msg.verb == "deleted" {
		if reload := m.loadContextsReload(); reload != nil {
			cmds = append(cmds, reload)
		}
	}
	return m, tea.Batch(cmds...)
}

// buildLocalClusterDeleteConfirmView translates the Model's
// delete-confirm sub-screen state into the renderer's view payload.
// When deleteRow is out of range (defensive — should not happen with
// the d-key handler's bounds check) the view renders with empty
// fields so the overlay still shows the "type DELETE" prompt.
func (m Model) buildLocalClusterDeleteConfirmView() ui.LocalClusterDeleteConfirmView {
	if m.localClusterState.deleteRow >= len(m.localClusterState.clusters) {
		return ui.LocalClusterDeleteConfirmView{Buffer: m.localClusterState.deleteBuf}
	}
	r := m.localClusterState.clusters[m.localClusterState.deleteRow]
	return ui.LocalClusterDeleteConfirmView{
		Provider:    r.Provider,
		Name:        r.Name,
		ContextName: r.ContextName,
		Buffer:      m.localClusterState.deleteBuf,
	}
}

// buildLocalClusterWizardView translates Model wizard state into the
// renderer's view payload. Step is derived arithmetically from the
// iota offset between the current wizard screen and the first wizard
// screen — keeps the breadcrumb in sync with the dispatch even if a
// new sub-screen is inserted (no per-case switch to keep updated).
// Defaults to 1 when the screen is not a wizard sub-screen, but in
// practice this builder is only called when the manager dispatch
// already routed to the wizard renderer.
func (m Model) buildLocalClusterWizardView() ui.LocalClusterWizardView {
	step := 1
	if s := m.localClusterState.screen; s >= localClusterScreenWizardProvider && s <= localClusterScreenWizardConfirm {
		step = int(s-localClusterScreenWizardProvider) + 1
	}
	w := m.localClusterState.wizard
	return ui.LocalClusterWizardView{
		Step:               step,
		Provider:           w.provider,
		ProviderCursor:     w.providerCur,
		InstalledProviders: w.installedSet,
		Name:               w.name,
		NameErr:            w.nameErr,
		Version:            w.version,
		VersionErr:         w.versionErr,
		Nodes:              w.nodesStr,
		NodesErr:           w.nodesErr,
	}
}
