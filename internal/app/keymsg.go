package app

import (
	tea "charm.land/bubbletea/v2"
)

// keyPressText builds a printable key-press message for text, mirroring what
// the terminal delivers when the user types those characters.
//
// Bubble Tea v2 splits what v1 packed into KeyMsg.Runes across two fields:
// Code carries the key that was pressed, Text the characters it produced.
// Both must be set or msg.String() reports the wrong key — with Text empty it
// falls back to the modifier-prefixed keystroke form.
func keyPressText(text string) tea.KeyPressMsg {
	k := tea.KeyPressMsg{Text: text}
	for _, r := range text {
		k.Code = r
		break
	}
	return k
}

// keyPressRunes is keyPressText for callers that already hold a rune slice.
func keyPressRunes(runes []rune) tea.KeyPressMsg {
	return keyPressText(string(runes))
}

// pasteKey encodes bracketed-paste content as a key-press message carrying no
// key code. A real keystroke always reports the code of the key pressed, so a
// zero Code with non-empty Text means "pasted text" and nothing else.
//
// Bubble Tea v1 flagged paste on the key message itself; v2 delivers it as a
// separate tea.PasteMsg. Re-encoding it as a key message lets paste travel the
// existing focus-precedence chain in handleKey and reach whichever input owns
// the keyboard, rather than duplicating that chain for paste alone.
func pasteKey(text string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: text}
}

// isPaste reports whether msg carries bracketed-paste content rather than a
// keystroke. Text-input handlers check this before their normal key handling.
func isPaste(msg tea.KeyPressMsg) bool {
	return msg.Code == 0 && msg.Text != ""
}
