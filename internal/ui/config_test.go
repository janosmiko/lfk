package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultKeybindings_CriticalDefaults(t *testing.T) {
	kb := DefaultKeybindings()
	// Verify critical defaults
	assert.Equal(t, "h", kb.Left)
	assert.Equal(t, "l", kb.Right)
	assert.Equal(t, "j", kb.Down)
	assert.Equal(t, "k", kb.Up)
	assert.Equal(t, "L", kb.Logs)
	assert.Equal(t, "v", kb.Describe)
	assert.Equal(t, "D", kb.Delete)
	assert.Equal(t, "ctrl+g", kb.FinalizerSearch)
	assert.Equal(t, " ", kb.ToggleSelect)
	assert.Equal(t, "t", kb.NewTab)
	assert.Equal(t, "m", kb.SetMark)
}

func TestMergeKeybindings_OverridesNonEmpty(t *testing.T) {
	dst := DefaultKeybindings()
	src := Keybindings{
		Logs:     "l",
		Describe: "d",
	}
	MergeKeybindings(&dst, &src)
	assert.Equal(t, "l", dst.Logs)     // overridden
	assert.Equal(t, "d", dst.Describe) // overridden
	assert.Equal(t, "h", dst.Left)     // unchanged (src empty)
	assert.Equal(t, "D", dst.Delete)   // unchanged (src empty)
}

func TestMergeKeybindings_EmptySourceNoChange(t *testing.T) {
	dst := DefaultKeybindings()
	src := Keybindings{} // all empty
	MergeKeybindings(&dst, &src)
	assert.Equal(t, DefaultKeybindings(), dst) // unchanged
}

func TestMergeKeybindings_AllFieldsOverridable(t *testing.T) {
	dst := DefaultKeybindings()
	src := Keybindings{
		Left: "a", Right: "b", Down: "c", Up: "d",
		Help: "F1", Filter: "ctrl+f",
		ActionMenu: "z", NewTab: "ctrl+t",
	}
	MergeKeybindings(&dst, &src)
	assert.Equal(t, "a", dst.Left)
	assert.Equal(t, "b", dst.Right)
	assert.Equal(t, "F1", dst.Help)
	assert.Equal(t, "ctrl+f", dst.Filter)
	assert.Equal(t, "z", dst.ActionMenu)
	assert.Equal(t, "ctrl+t", dst.NewTab)
}

func TestDefaultAbbreviations_KnownEntries(t *testing.T) {
	abbr := DefaultAbbreviations()
	assert.Equal(t, "persistentvolumeclaim", abbr["pvc"])
	assert.Equal(t, "deployment", abbr["deploy"])
	assert.Equal(t, "pod", abbr["po"])
	assert.NotEmpty(t, abbr)
}

func TestColumnsForKind_CaseInsensitive(t *testing.T) {
	orig := ConfigResourceColumns
	t.Cleanup(func() { ConfigResourceColumns = orig })

	ConfigResourceColumns = map[string][]string{
		"pod": {"name", "status"},
	}

	assert.Equal(t, []string{"name", "status"}, ColumnsForKind("Pod", ""))
	assert.Equal(t, []string{"name", "status"}, ColumnsForKind("pod", ""))
	assert.Nil(t, ColumnsForKind("Deployment", ""))
	assert.Nil(t, ColumnsForKind("", ""))
}
