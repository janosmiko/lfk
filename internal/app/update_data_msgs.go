package app

import (
	"maps"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateSecretDataLoaded(msg secretDataLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error loading secret: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.secretData = msg.data
	// Snapshot the original data for dirty detection on save.
	m.secretDataOriginal = make(map[string]string, len(msg.data.Data))
	maps.Copy(m.secretDataOriginal, msg.data.Data)
	m.secretCursor = 0
	m.secretRevealed = make(map[string]bool)
	m.secretAllRevealed = false
	m.secretEditing = false
	m.secretEditColumn = -1
	m.resetEditorSearch()
	m.overlay = overlaySecretEditor
	return m, nil
}

func (m Model) updateSecretSaved(msg secretSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error saving secret: ", msg.err)
	} else {
		m.setStatusMessage("Secret saved", false)
		m.overlay = overlayNone

		// Invalidate the preview cache for this secret so the next hover
		// re-fetches fresh data after the save.
		if sel := m.selectedMiddleItem(); sel != nil {
			kctx := m.nav.Context
			ns := m.resolveNamespace()
			if sel.Namespace != "" {
				ns = sel.Namespace
			}
			key := secretPreviewCacheKey(kctx, ns, sel.Name)
			delete(m.secretPreviewCache, key)
		}
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateConfigMapDataLoaded(msg configMapDataLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error loading configmap: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.configMapData = msg.data
	// Snapshot the original data for dirty detection on save.
	m.configMapDataOriginal = make(map[string]string, len(msg.data.Data))
	maps.Copy(m.configMapDataOriginal, msg.data.Data)
	m.configMapCursor = 0
	m.configMapEditing = false
	m.configMapEditColumn = -1
	m.resetEditorSearch()
	m.overlay = overlayConfigMapEditor
	return m, nil
}

func (m Model) updateConfigMapSaved(msg configMapSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error saving configmap: ", msg.err)
	} else {
		m.setStatusMessage("ConfigMap saved", false)
		m.overlay = overlayNone
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateLabelDataLoaded(msg labelDataLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error loading labels: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.labelData = msg.data
	// Snapshot both maps for dirty detection on save.
	m.labelLabelsOriginal = make(map[string]string, len(msg.data.Labels))
	maps.Copy(m.labelLabelsOriginal, msg.data.Labels)
	m.labelAnnotationsOriginal = make(map[string]string, len(msg.data.Annotations))
	maps.Copy(m.labelAnnotationsOriginal, msg.data.Annotations)
	m.labelCursor = 0
	m.labelTab = 0
	m.labelEditing = false
	m.labelEditColumn = -1
	m.resetEditorSearch()
	m.overlay = overlayLabelEditor
	return m, nil
}

func (m Model) updateLabelSaved(msg labelSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error saving labels: ", msg.err)
	} else {
		m.setStatusMessage("Labels/annotations saved", false)
		m.overlay = overlayNone
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}
