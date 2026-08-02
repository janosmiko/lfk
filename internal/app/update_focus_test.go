package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestUpdateBlurClearsFocus(t *testing.T) {
	m := Model{focused: true}
	out, _ := m.updateBlur(tea.BlurMsg{})
	if out.(Model).focused {
		t.Fatal("blur must clear focused")
	}
}

func TestUpdateFocusSetsFocusAndResetsIdle(t *testing.T) {
	old := time.Now().Add(-time.Hour)
	m := Model{focused: false, lastInputAt: old, watchThrottle: true, watchMode: true, watchInterval: time.Second, backgroundWatchInterval: 30 * time.Second}
	out, _ := m.updateFocus(tea.FocusMsg{})
	got := out.(Model)
	if !got.focused {
		t.Fatal("focus must set focused")
	}
	if !got.lastInputAt.After(old) {
		t.Fatal("focus must reset the idle clock")
	}
	if got.suppressBgtasks {
		t.Fatal("suppressBgtasks must not leak past updateFocus")
	}
}
