package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionsForKind_PodHasResize(t *testing.T) {
	items := ActionsForKind("Pod")

	var resize *ActionMenuItem
	seenKeys := map[string]string{}
	for i, it := range items {
		if it.Label == "Resize" {
			resize = &items[i]
		}
		if _, dup := seenKeys[it.Key]; dup {
			t.Fatalf("Pod action key %q used by more than one action", it.Key)
		}
		seenKeys[it.Key] = it.Label
	}

	require.NotNil(t, resize, "Pod actions must include a Resize entry")
	assert.Equal(t, "r", resize.Key)
}
