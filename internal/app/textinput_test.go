package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTextInput_Insert(t *testing.T) {
	ti := TextInput{}
	ti.Insert("hello")
	assert.Equal(t, "hello", ti.Value)
	assert.Equal(t, 5, ti.Cursor)

	ti.Insert(" world")
	assert.Equal(t, "hello world", ti.Value)
	assert.Equal(t, 11, ti.Cursor)

	// Insert in the middle.
	ti.Cursor = 5
	ti.Insert(",")
	assert.Equal(t, "hello, world", ti.Value)
	assert.Equal(t, 6, ti.Cursor)
}

func TestTextInput_Backspace(t *testing.T) {
	ti := TextInput{Value: "hello", Cursor: 5}
	ti.Backspace()
	assert.Equal(t, "hell", ti.Value)
	assert.Equal(t, 4, ti.Cursor)

	// Backspace at beginning does nothing.
	ti.Cursor = 0
	ti.Backspace()
	assert.Equal(t, "hell", ti.Value)
	assert.Equal(t, 0, ti.Cursor)

	// Backspace in the middle.
	ti = TextInput{Value: "abcde", Cursor: 3}
	ti.Backspace()
	assert.Equal(t, "abde", ti.Value)
	assert.Equal(t, 2, ti.Cursor)
}

func TestTextInput_DeleteWord(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		cursor     int
		wantValue  string
		wantCursor int
	}{
		{
			name:       "empty",
			value:      "",
			cursor:     0,
			wantValue:  "",
			wantCursor: 0,
		},
		{
			name:       "single word at end",
			value:      "hello",
			cursor:     5,
			wantValue:  "",
			wantCursor: 0,
		},
		{
			name:       "trailing space",
			value:      "hello ",
			cursor:     6,
			wantValue:  "",
			wantCursor: 0,
		},
		{
			name:       "two words at end",
			value:      "hello world",
			cursor:     11,
			wantValue:  "hello ",
			wantCursor: 6,
		},
		{
			name:       "cursor in middle of text",
			value:      "hello beautiful world",
			cursor:     15,
			wantValue:  "hello  world",
			wantCursor: 6,
		},
		{
			name:       "cursor at beginning",
			value:      "hello",
			cursor:     0,
			wantValue:  "hello",
			wantCursor: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ti := TextInput{Value: tt.value, Cursor: tt.cursor}
			ti.DeleteWord()
			assert.Equal(t, tt.wantValue, ti.Value)
			assert.Equal(t, tt.wantCursor, ti.Cursor)
		})
	}
}

func TestTextInput_Home(t *testing.T) {
	ti := TextInput{Value: "hello", Cursor: 3}
	ti.Home()
	assert.Equal(t, 0, ti.Cursor)
	assert.Equal(t, "hello", ti.Value)
}

func TestTextInput_End(t *testing.T) {
	ti := TextInput{Value: "hello", Cursor: 2}
	ti.End()
	assert.Equal(t, 5, ti.Cursor)
	assert.Equal(t, "hello", ti.Value)
}

func TestTextInput_Left(t *testing.T) {
	ti := TextInput{Value: "hello", Cursor: 3}
	ti.Left()
	assert.Equal(t, 2, ti.Cursor)

	// Left at beginning does nothing.
	ti.Cursor = 0
	ti.Left()
	assert.Equal(t, 0, ti.Cursor)
}

func TestTextInput_Right(t *testing.T) {
	ti := TextInput{Value: "hello", Cursor: 3}
	ti.Right()
	assert.Equal(t, 4, ti.Cursor)

	// Right at end does nothing.
	ti.Cursor = 5
	ti.Right()
	assert.Equal(t, 5, ti.Cursor)
}

func TestTextInput_Set(t *testing.T) {
	ti := TextInput{}
	ti.Set("new value")
	assert.Equal(t, "new value", ti.Value)
	assert.Equal(t, 9, ti.Cursor)

	// Set overwrites existing.
	ti.Set("replaced")
	assert.Equal(t, "replaced", ti.Value)
	assert.Equal(t, 8, ti.Cursor)
}

func TestTextInput_Clear(t *testing.T) {
	ti := TextInput{Value: "hello", Cursor: 3}
	ti.Clear()
	assert.Equal(t, "", ti.Value)
	assert.Equal(t, 0, ti.Cursor)
}

func TestTextInput_String(t *testing.T) {
	ti := TextInput{Value: "test"}
	assert.Equal(t, "test", ti.String())
}

func TestTextInput_CursorLeftRight(t *testing.T) {
	ti := TextInput{Value: "hello world", Cursor: 5}
	assert.Equal(t, "hello", ti.CursorLeft())
	assert.Equal(t, " world", ti.CursorRight())

	// At beginning.
	ti.Cursor = 0
	assert.Equal(t, "", ti.CursorLeft())
	assert.Equal(t, "hello world", ti.CursorRight())

	// At end.
	ti.Cursor = 11
	assert.Equal(t, "hello world", ti.CursorLeft())
	assert.Equal(t, "", ti.CursorRight())
}

func TestTextInput_InsertNewline(t *testing.T) {
	ti := TextInput{Value: "line1", Cursor: 5}
	ti.Insert("\n")
	assert.Equal(t, "line1\n", ti.Value)
	assert.Equal(t, 6, ti.Cursor)

	ti.Insert("line2")
	assert.Equal(t, "line1\nline2", ti.Value)
}
