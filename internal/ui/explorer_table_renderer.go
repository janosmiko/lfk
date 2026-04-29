package ui

import (
	"sort"
	"strings"
	"time"
	"unsafe"

	"github.com/janosmiko/lfk/internal/model"
)

// ageBucketSeconds bounds the staleness window for LiveAge values baked into
// cached non-cursor row strings. Bumping the bucket invalidates the cache, so
// ages tick on screen even when the data slice is otherwise unchanged (e.g.
// watch mode is off).
const ageBucketSeconds = 30

// TableRenderer caches the non-cursor row strings and column-width layout
// across renders, keyed by an input fingerprint. Cursor rows are always
// re-rendered.
//
// Concurrency: Render mutates package-level globals (ActiveRowCache,
// ActiveTableLayout) and assumes the caller is the single-threaded Bubble
// Tea View() goroutine. Calling Render from multiple goroutines races on
// those globals.
type TableRenderer struct {
	fp     tableFingerprint
	rows   map[int]string
	layout TableLayoutCache
}

type tableFingerprint struct {
	itemsPtr     uintptr
	itemsLen     int
	middleRev    uint64
	selRev       uint64
	ageBucket    int64
	width        int
	height       int
	highlight    string
	hiddenCols   string
	columnOrder  string
	sessionCols  string
	contextLabel string
	iconMode     string
	noColor      bool
}

func NewTableRenderer() *TableRenderer {
	return &TableRenderer{rows: make(map[int]string)}
}

func (r *TableRenderer) Render(headerLabel string, items []model.Item, cursor int, width, height int, loading bool, spinnerView string, errMsg string, middleRev, selRev uint64) string {
	fp := tableFingerprint{
		itemsPtr:     itemsHeaderPtr(items),
		itemsLen:     len(items),
		middleRev:    middleRev,
		selRev:       selRev,
		ageBucket:    time.Now().Unix() / ageBucketSeconds,
		width:        width,
		height:       height,
		highlight:    ActiveHighlightQuery,
		hiddenCols:   serializeBoolSet(ActiveHiddenBuiltinColumns),
		columnOrder:  strings.Join(ActiveColumnOrder, "|"),
		sessionCols:  strings.Join(ActiveSessionColumns, "|"),
		contextLabel: ActiveContext,
		iconMode:     IconMode,
		noColor:      ConfigNoColor,
	}
	if r.fp != fp {
		r.fp = fp
		clear(r.rows)
		r.layout = TableLayoutCache{}
	}
	prevCache := ActiveRowCache
	prevLayout := ActiveTableLayout
	defer func() {
		ActiveRowCache = prevCache
		ActiveTableLayout = prevLayout
	}()
	ActiveRowCache = r.rows
	ActiveTableLayout = &r.layout
	return RenderTable(headerLabel, items, cursor, width, height, loading, spinnerView, errMsg)
}

func itemsHeaderPtr(items []model.Item) uintptr {
	if len(items) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&items[0]))
}

func serializeBoolSet(m map[string]bool) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return strings.Join(keys, "|")
}
