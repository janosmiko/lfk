package app

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

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
		_, n := utf8.DecodeLastRuneInString(t.Value[:t.Cursor])
		t.Value = t.Value[:t.Cursor-n] + t.Value[t.Cursor:]
		t.Cursor -= n
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

// LineHome moves the cursor to the start of the current line — i.e.
// the byte just after the previous '\n', or 0 if there is no
// preceding newline. Used by the multi-line edit pane (ctrl+a /
// vim-`0`) so single-line readline-style buffers (the / search input,
// numeric inputs) can keep the buffer-wide Home() while edit-pane
// values get line-scoped behaviour.
func (t *TextInput) LineHome() {
	t.Cursor = strings.LastIndex(t.Value[:t.Cursor], "\n") + 1
}

// LineEnd moves the cursor to the end of the current line — i.e. the
// byte before the next '\n', or len(value) if no following newline.
// Counterpart to LineHome.
func (t *TextInput) LineEnd() {
	if idx := strings.Index(t.Value[t.Cursor:], "\n"); idx != -1 {
		t.Cursor += idx
		return
	}
	t.Cursor = len(t.Value)
}

// Left moves the cursor one position to the left.
func (t *TextInput) Left() {
	if t.Cursor > 0 {
		_, n := utf8.DecodeLastRuneInString(t.Value[:t.Cursor])
		t.Cursor -= n
	}
}

// Right moves the cursor one position to the right.
func (t *TextInput) Right() {
	if t.Cursor < len(t.Value) {
		_, n := utf8.DecodeRuneInString(t.Value[t.Cursor:])
		t.Cursor += n
	}
}

// cellOffset maps display column col to a rune-aligned byte offset in
// line. Wide runes (CJK) take two terminal cells.
func cellOffset(line string, col int) int {
	w := 0
	for i, r := range line {
		rw := ansi.StringWidth(string(r))
		if w+rw > col {
			return i
		}
		w += rw
	}
	return len(line)
}

// Up moves the cursor to the same display column on the previous
// `\n`-delimited line. No-op when already on the first line.
// If the previous line is shorter than the current column, lands at
// its end. Used by the multi-line edit pane (Secret/ConfigMap/Label
// editors) so arrow-up navigates between hard-wrapped lines.
func (t *TextInput) Up() {
	lineStart := strings.LastIndex(t.Value[:t.Cursor], "\n") + 1
	if lineStart == 0 {
		return // already on first line
	}
	col := ansi.StringWidth(t.Value[lineStart:t.Cursor])
	prevLineEnd := lineStart - 1 // index of the '\n'
	prevLineStart := strings.LastIndex(t.Value[:prevLineEnd], "\n") + 1
	t.Cursor = prevLineStart + cellOffset(t.Value[prevLineStart:prevLineEnd], col)
}

// Down moves the cursor to the same display column on the next
// `\n`-delimited line. No-op when already on the last line.
func (t *TextInput) Down() {
	lineStart := strings.LastIndex(t.Value[:t.Cursor], "\n") + 1
	col := ansi.StringWidth(t.Value[lineStart:t.Cursor])
	nextNL := strings.Index(t.Value[t.Cursor:], "\n")
	if nextNL == -1 {
		return // no next line
	}
	nextLineStart := t.Cursor + nextNL + 1
	nextLineEnd := len(t.Value)
	if idx := strings.Index(t.Value[nextLineStart:], "\n"); idx != -1 {
		nextLineEnd = nextLineStart + idx
	}
	t.Cursor = nextLineStart + cellOffset(t.Value[nextLineStart:nextLineEnd], col)
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
