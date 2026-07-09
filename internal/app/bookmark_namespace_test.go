package app

import (
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
)

// TestHandleKeyOpenMarks_DefaultsToLoadNamespace verifies the bookmark
// overlay opens with load-namespace ON, so a plain Enter/slot jump replays
// the bookmark's saved scope without any Tab press.
func TestHandleKeyOpenMarks_DefaultsToLoadNamespace(t *testing.T) {
	m := baseFinalModel()
	m.bookmarkLoadNamespace = false // simulate a stale prior value

	rm := m.handleKeyOpenMarks()

	assert.Equal(t, overlayBookmarks, rm.overlay)
	assert.True(t, rm.bookmarkLoadNamespace, "open must default to loading the saved namespace")
}

func TestBookmarkNamespaceLabel(t *testing.T) {
	tests := []struct {
		name string
		bm   model.Bookmark
		want string
	}{
		{"unscoped is all namespaces", model.Bookmark{}, "all namespaces"},
		{"single namespace field", model.Bookmark{Namespace: "prod"}, "prod"},
		{"single via list", model.Bookmark{Namespaces: []string{"prod"}}, "prod"},
		{"multi sorted", model.Bookmark{Namespaces: []string{"web", "api"}}, "api,web"},
		{"negated set", model.Bookmark{Namespaces: []string{"kube-system"}, NsSelectionNegated: true}, "!kube-system"},
		{"capped at three", model.Bookmark{Namespaces: []string{"a", "b", "c", "d", "e"}}, "a,b,c +2 more"},
		{"capped negated", model.Bookmark{Namespaces: []string{"a", "b", "c", "d"}, NsSelectionNegated: true}, "!a,!b,!c +1 more excl"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, bookmarkNamespaceLabel(tt.bm))
		})
	}
}

// TestRenderBookmarkOverlay_ShowsNamespace verifies each bookmark row
// carries its saved namespace, and the opt-out chip appears only when the
// user has toggled loading off.
func TestRenderBookmarkOverlay_ShowsNamespace(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	m.bookmarkLoadNamespace = true
	m.bookmarks = []model.Bookmark{
		{Slot: "p", Name: "prod pods", Namespace: "production"},
		{Slot: "a", Name: "all", Namespace: ""},
	}

	view := stripANSI(renderBookmarkOverlay(m))
	assert.Contains(t, view, "ns: production")
	assert.Contains(t, view, "ns: all namespaces")
	assert.NotContains(t, view, "KEEP CURRENT NS", "no chip while loading (the default)")

	m.bookmarkLoadNamespace = false
	optedOut := stripANSI(renderBookmarkOverlay(m))
	assert.Contains(t, optedOut, "KEEP CURRENT NS", "chip shows when the user opts out")
}

// sanity: label helper never panics on a nil Namespaces slice.
func TestBookmarkNamespaceLabel_NilSlice(t *testing.T) {
	assert.Equal(t, "all namespaces", bookmarkNamespaceLabel(model.Bookmark{Namespaces: nil}))
	assert.False(t, strings.Contains(bookmarkNamespaceLabel(model.Bookmark{Namespace: "x"}), "more"))
}
