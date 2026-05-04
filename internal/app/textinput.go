package app

import "strings"

// TextInput is a simple readline-style text input with cursor positioning.
// It supports insert, delete, word-delete, and cursor movement operations.
type TextInput struct {
	Value  string
	Cursor int
}

// Insert inserts a string at the current cursor position.
func (t *TextInput) Insert(s string) {
	t.Value = t.Value[:t.Cursor] + s + t.Value[t.Cursor:]
	t.Cursor += len(s)
}

// Backspace deletes the character before the cursor.
func (t *TextInput) Backspace() {
	if t.Cursor > 0 {
		t.Value = t.Value[:t.Cursor-1] + t.Value[t.Cursor:]
		t.Cursor--
	}
}

// DeleteWord deletes the word before the cursor (same logic as deleteWordBackward).
func (t *TextInput) DeleteWord() {
	if t.Cursor == 0 {
		return
	}
	// Work backwards from cursor position.
	i := t.Cursor - 1
	// Trim trailing spaces first.
	for i >= 0 && t.Value[i] == ' ' {
		i--
	}
	// Then trim the word.
	for i >= 0 && t.Value[i] != ' ' {
		i--
	}
	t.Value = t.Value[:i+1] + t.Value[t.Cursor:]
	t.Cursor = i + 1
}

// DeleteLine deletes everything before the cursor (Unix ctrl+u behavior).
func (t *TextInput) DeleteLine() {
	if t.Cursor == 0 {
		return
	}
	t.Value = t.Value[t.Cursor:]
	t.Cursor = 0
}

// Home moves the cursor to the beginning of the input.
func (t *TextInput) Home() {
	t.Cursor = 0
}

// End moves the cursor to the end of the input.
func (t *TextInput) End() {
	t.Cursor = len(t.Value)
}

// Left moves the cursor one position to the left.
func (t *TextInput) Left() {
	if t.Cursor > 0 {
		t.Cursor--
	}
}

// Right moves the cursor one position to the right.
func (t *TextInput) Right() {
	if t.Cursor < len(t.Value) {
		t.Cursor++
	}
}

// Up moves the cursor to the same byte-column on the previous
// `\n`-delimited line. No-op when already on the first line.
// If the previous line is shorter than the current column, lands at
// its end. Used by the multi-line edit pane (Secret/ConfigMap/Label
// editors) so arrow-up navigates between hard-wrapped lines.
func (t *TextInput) Up() {
	lineStart := strings.LastIndex(t.Value[:t.Cursor], "\n") + 1
	col := t.Cursor - lineStart
	if lineStart == 0 {
		return // already on first line
	}
	prevLineEnd := lineStart - 1 // index of the '\n'
	prevLineStart := strings.LastIndex(t.Value[:prevLineEnd], "\n") + 1
	prevLineLen := prevLineEnd - prevLineStart
	if col > prevLineLen {
		col = prevLineLen
	}
	t.Cursor = prevLineStart + col
}

// Down moves the cursor to the same byte-column on the next
// `\n`-delimited line. No-op when already on the last line.
func (t *TextInput) Down() {
	lineStart := strings.LastIndex(t.Value[:t.Cursor], "\n") + 1
	col := t.Cursor - lineStart
	nextNL := strings.Index(t.Value[t.Cursor:], "\n")
	if nextNL == -1 {
		return // no next line
	}
	nextLineStart := t.Cursor + nextNL + 1
	nextLineLen := len(t.Value) - nextLineStart
	if idx := strings.Index(t.Value[nextLineStart:], "\n"); idx != -1 {
		nextLineLen = idx
	}
	if col > nextLineLen {
		col = nextLineLen
	}
	t.Cursor = nextLineStart + col
}

// Set replaces the entire value and moves the cursor to the end.
func (t *TextInput) Set(s string) {
	t.Value = s
	t.Cursor = len(s)
}

// Clear empties the input and resets the cursor.
func (t *TextInput) Clear() {
	t.Value = ""
	t.Cursor = 0
}

// String returns the current value (implements fmt.Stringer).
func (t *TextInput) String() string {
	return t.Value
}

// CursorLeft returns the text to the left of the cursor.
func (t *TextInput) CursorLeft() string {
	return t.Value[:t.Cursor]
}

// CursorRight returns the text to the right of the cursor.
func (t *TextInput) CursorRight() string {
	return t.Value[t.Cursor:]
}
