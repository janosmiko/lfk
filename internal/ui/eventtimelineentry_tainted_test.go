package ui

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEventTimelineEntryHasNoRawStringFields is the internal/ui mirror of the
// k8s.EventInfo guard. The two structs are copies of each other so internal/ui
// need not import internal/k8s, which means a field added to one and mirrored
// into the other as a plain string would reopen the leak on the render side
// only. Guard both ends.
func TestEventTimelineEntryHasNoRawStringFields(t *testing.T) {
	for f := range reflect.TypeFor[EventTimelineEntry]().Fields() {
		assert.NotEqual(t, reflect.String, f.Type.Kind(),
			"EventTimelineEntry.%s is a raw string; cluster-controlled text must be tainted.String", f.Name)
	}
}
