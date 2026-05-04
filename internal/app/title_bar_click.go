package app

import (
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// titleBarLayout records click-target regions on the title bar so the
// mouse handler can map (x, 0) to a known label. Populated during
// renderTitleBar each frame and consumed by handleTitleBarClick. We
// only track the namespace badge today; future entries (read-only
// toggle, watch indicator, etc.) plug in here.
type titleBarLayout struct {
	nsStartX, nsEndX int
}

// activeTitleBarLayout is read at click time and written every frame
// via renderTitleBar. Production has a single update goroutine, but
// view/render tests call renderTitleBar from goroutines marked
// t.Parallel(), so we serialize the access with a small mutex.
var (
	activeTitleBarLayout   titleBarLayout
	activeTitleBarLayoutMu sync.RWMutex
)

func setTitleBarLayout(l titleBarLayout) {
	activeTitleBarLayoutMu.Lock()
	activeTitleBarLayout = l
	activeTitleBarLayoutMu.Unlock()
}

func getTitleBarLayout() titleBarLayout {
	activeTitleBarLayoutMu.RLock()
	l := activeTitleBarLayout
	activeTitleBarLayoutMu.RUnlock()
	return l
}

// handleTitleBarClick routes a click on the title bar (y == 0) to a
// known clickable label. ok=true means the click was consumed; the
// caller should return the resulting model/cmd. ok=false means the
// click landed on title-bar real estate that has no associated action,
// in which case the caller should treat it as a no-op rather than
// falling through to the column-header sort path — clicks on the title
// bar shouldn't accidentally re-sort the table beneath it.
func (m Model) handleTitleBarClick(x int) (tea.Model, tea.Cmd, bool) {
	r := getTitleBarLayout()
	if r.nsEndX > r.nsStartX && x >= r.nsStartX && x < r.nsEndX {
		mdl, cmd := m.handleKeyNamespaceSelector()
		return mdl, cmd, true
	}
	return m, nil, false
}
