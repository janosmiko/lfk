package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// TestLogViewStateCopy validates the deep-copy semantics tabs.go relies on:
// the value-typed slices (lines, multiItems, containers, selectedContainers)
// are cloned, while the channel/cancel/searchHistory handles are shared by
// assignment (matching the pre-extraction save/restore behaviour).
func TestLogViewStateCopy(t *testing.T) {
	ch := make(chan string)
	cancelled := false
	cancel := context.CancelFunc(func() { cancelled = true })
	src := logViewState{
		lines:              []string{"a", "b"},
		scroll:             5,
		follow:             true,
		multiItems:         []model.Item{{Name: "p1"}},
		containers:         []string{"main", "side"},
		selectedContainers: []string{"main"},
		ch:                 ch,
		cancel:             cancel,
		title:              "Logs",
		cursor:             1,
	}

	cp := src.copy()
	assert.Equal(t, []string{"a", "b"}, cp.lines)
	assert.Equal(t, "Logs", cp.title)

	// Slices are cloned.
	cp.lines[0] = "x"
	assert.Equal(t, "a", src.lines[0], "lines must be cloned")
	cp.containers[0] = "x"
	assert.Equal(t, "main", src.containers[0], "containers must be cloned")
	cp.selectedContainers = append(cp.selectedContainers, "extra")
	assert.Len(t, src.selectedContainers, 1, "selectedContainers header must be independent")
	cp.multiItems[0].Name = "x"
	assert.Equal(t, "p1", src.multiItems[0].Name, "multiItems must be cloned")

	// Channel and cancel func are shared by value (live handles, not snapshots).
	assert.Equal(t, ch, cp.ch, "channel handle is shared")
	cp.cancel()
	assert.True(t, cancelled, "cancel func is shared")
}

// TestLogViewStateCopyNilFields ensures copy leaves nil slices nil.
func TestLogViewStateCopyNilFields(t *testing.T) {
	cp := logViewState{}.copy()
	assert.Nil(t, cp.lines)
	assert.Nil(t, cp.multiItems)
	assert.Nil(t, cp.containers)
	assert.Nil(t, cp.selectedContainers)
}
