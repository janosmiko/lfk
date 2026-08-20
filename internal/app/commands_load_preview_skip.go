package app

import "github.com/janosmiko/lfk/internal/model"

// itemResourceVersion reads metadata.resourceVersion from the item's raw
// object. Returns "" on any type-assertion miss so callers fail open.
func itemResourceVersion(it model.Item) string {
	meta, ok := it.Raw["metadata"].(map[string]any)
	if !ok {
		return ""
	}
	rv, _ := meta["resourceVersion"].(string)
	return rv
}

// previewFingerprintKey scopes SelectionKey to context + resource kind,
// matching itemCache/cacheFingerprints, so a same-named object elsewhere
// never matches this one's fingerprint.
func (m Model) previewFingerprintKey(sel *model.Item) string {
	if sel == nil {
		return ""
	}
	return m.navKey() + "/" + sel.SelectionKey()
}

// previewContentUnchanged reports whether sel's resourceVersion matches the
// fingerprint recorded by a prior call. An empty resourceVersion is never
// eligible for skipping (fail open).
func (m Model) previewContentUnchanged(sel *model.Item) bool {
	if sel == nil {
		return false
	}
	rv := itemResourceVersion(*sel)
	if rv == "" {
		return false
	}
	return m.previewContentFingerprints[m.previewFingerprintKey(sel)] == rv
}

// recordPreviewContentFingerprint stores sel's resourceVersion for the next
// previewContentUnchanged check. Only the currently hovered item's verdict
// ever matters, so any other entry is dropped to keep the store at size 1.
func (m Model) recordPreviewContentFingerprint(sel *model.Item) {
	if sel == nil {
		return
	}
	rv := itemResourceVersion(*sel)
	if rv == "" {
		return
	}
	key := m.previewFingerprintKey(sel)
	for k := range m.previewContentFingerprints {
		if k != key {
			delete(m.previewContentFingerprints, k)
		}
	}
	m.previewContentFingerprints[key] = rv
}

// clearPreviewContentFingerprint re-arms the skipped fetches after one
// failed. The record happens at dispatch, so a failure would otherwise keep
// matching the unchanged resourceVersion and the retry never runs.
func (m Model) clearPreviewContentFingerprint() {
	clear(m.previewContentFingerprints)
}
