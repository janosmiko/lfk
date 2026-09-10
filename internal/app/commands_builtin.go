package app

// openFinalizerSearch opens the finalizer search overlay in search prompt mode.
// The user types a pattern and presses enter to start scanning.
func (m *Model) openFinalizerSearch() {
	m.finalizerSearch.pattern = ""
	m.finalizerSearch.results = nil
	m.finalizerSearch.selected = make(map[string]bool)
	m.finalizerSearch.cursor = 0
	m.finalizerSearch.filter = ""
	m.finalizerSearch.filterActive = true // start in search/input mode
	m.finalizerSearch.loading = false
	m.overlay = overlayFinalizerSearch
}
