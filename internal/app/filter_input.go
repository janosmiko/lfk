package app

import "strings"

// filterAction describes the outcome of a key press inside a filter input.
type filterAction int

const (
	// filterContinue means the key was a text edit (insert, delete, word-delete).
	// The caller should typically reset its overlay cursor to 0.
	filterContinue filterAction = iota

	// filterEscape means the user pressed Esc.
	// The caller decides whether to clear the filter text and/or exit filter mode.
	filterEscape

	// filterAccept means the user pressed Enter.
	// The caller decides whether to keep or discard the filter text.
	filterAccept

	// filterClose means the user pressed Ctrl+C.
	// The caller should close the tab or quit.
	filterClose

	// filterNavigate means the key was a cursor movement (home/end/left/right).
	// No text was changed; the caller typically does nothing extra.
	filterNavigate

	// filterPasteMultiline means the input was a multi-line paste.
	// The caller should show a confirmation before inserting.
	// Returned by handlePastedText when the pasted text contains newlines.
	filterPasteMultiline

	// filterIgnored means the key was not handled by the filter input.
	filterIgnored
)

// FilterInput provides a uniform interface for text input fields used in
// overlay filter/search modes. It abstracts over TextInput structs and
// raw string fields so all filter handlers can share one implementation.
//
// The interface covers only mutation operations that handleFilterKey needs.
// Callers read the current text value directly from their own fields (e.g.
// TextInput.Value or the raw string) rather than through this interface.
type FilterInput interface {
	Insert(s string)
	Backspace()
	DeleteWord()
	DeleteLine()
	Home()
	End()
	Left()
	Right()
	Clear()
}

// Verify TextInput satisfies FilterInput at compile time.
var _ FilterInput = (*TextInput)(nil)

// stringFilterInput adapts a raw *string field to the FilterInput interface.
// It supports append-at-end semantics for Insert and truncation for Backspace
// and DeleteWord. Cursor movement operations (Home, End, Left, Right) are
// no-ops since a raw string has no cursor position concept.
type stringFilterInput struct {
	ptr *string
}

// Verify stringFilterInput satisfies FilterInput at compile time.
var _ FilterInput = (*stringFilterInput)(nil)

func (s *stringFilterInput) Clear()      { *s.ptr = "" }
func (s *stringFilterInput) DeleteLine() { *s.ptr = "" } // no cursor: clears all
func (s *stringFilterInput) Home()       {}              // no-op: raw string has no cursor
func (s *stringFilterInput) End()        {}              // no-op: raw string has no cursor
func (s *stringFilterInput) Left()       {}              // no-op: raw string has no cursor
func (s *stringFilterInput) Right()      {}              // no-op: raw string has no cursor

func (s *stringFilterInput) Insert(ch string) {
	*s.ptr += ch
}

func (s *stringFilterInput) Backspace() {
	v := *s.ptr
	if len(v) > 0 {
		*s.ptr = v[:len(v)-1]
	}
}

func (s *stringFilterInput) DeleteWord() {
	v := *s.ptr
	if len(v) == 0 {
		return
	}
	i := len(v) - 1
	// Trim trailing spaces first.
	for i >= 0 && v[i] == ' ' {
		i--
	}
	// Then trim the word.
	for i >= 0 && v[i] != ' ' {
		i--
	}
	*s.ptr = v[:i+1]
}

// triggerPasteConfirm sets up the paste confirmation overlay for multiline input.
func (m *Model) triggerPasteConfirm(text string, target FilterInput) {
	m.pendingPaste = text
	m.pasteTarget = target
	m.overlay = overlayPasteConfirm
}

// handlePastedText processes bracketed paste content for a filter input.
// Returns the filterAction: filterContinue if inserted, filterPasteMultiline
// if the caller must show a confirmation overlay first.
func handlePastedText(input FilterInput, runes []rune) filterAction {
	text := strings.TrimRight(string(runes), "\n")
	if text == "" {
		return filterIgnored
	}
	if strings.Contains(text, "\n") {
		return filterPasteMultiline
	}
	input.Insert(text)
	return filterContinue
}

// handleFilterKey processes a key message for a filter/search text input.
// It delegates text editing operations to the FilterInput and returns a
// filterAction indicating what happened. The caller is responsible for
// mode flag changes (esc/enter behavior varies per overlay) and any
// overlay-specific side effects (cursor resets, preview updates, etc.).
//
// NOTE: Callers must check msg.Paste BEFORE calling this function and
// use handlePastedText instead for paste events.
func handleFilterKey(input FilterInput, key string) filterAction {
	switch key {
	case "esc":
		return filterEscape
	case "enter":
		return filterAccept
	case "ctrl+c":
		return filterClose
	case "backspace":
		input.Backspace()
		return filterContinue
	case "ctrl+w":
		input.DeleteWord()
		return filterContinue
	case "ctrl+u":
		input.DeleteLine()
		return filterContinue
	case "ctrl+a":
		input.Home()
		return filterNavigate
	case "ctrl+e":
		input.End()
		return filterNavigate
	case "left":
		input.Left()
		return filterNavigate
	case "right":
		input.Right()
		return filterNavigate
	default:
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			input.Insert(key)
			return filterContinue
		}
		return filterIgnored
	}
}
