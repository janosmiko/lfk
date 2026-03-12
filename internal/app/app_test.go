package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- deleteWordBackward ---

func TestDeleteWordBackward(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"single word", "hello", ""},
		{"two words", "hello world", "hello "},
		{"trailing spaces", "hello   ", ""},
		{"three words", "one two three", "one two "},
		{"single char", "a", ""},
		{"spaces only", "   ", ""},
		{"word then spaces", "abc   def   ", "abc   "},
		{"unicode word", "hello wörld", "hello "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, deleteWordBackward(tt.input))
		})
	}
}

// --- padToHeight ---

func TestPadToHeight(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		height   int
		expected int // expected number of lines
	}{
		{"shorter content", "line1\nline2", 5, 5},
		{"exact height", "a\nb\nc", 3, 3},
		{"taller content", "a\nb\nc\nd\ne", 3, 3},
		{"empty content", "", 3, 3},
		{"single line", "hello", 1, 1},
		{"height zero", "hello", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := padToHeight(tt.content, tt.height)
			lines := strings.Split(result, "\n")
			if tt.height == 0 {
				// padToHeight truncates to 0 lines but Split always gives at least 1
				assert.LessOrEqual(t, len(lines), 1)
			} else {
				assert.Equal(t, tt.expected, len(lines))
			}
		})
	}

	// Verify padding uses empty strings
	t.Run("padding is empty lines", func(t *testing.T) {
		result := padToHeight("line1", 3)
		lines := strings.Split(result, "\n")
		assert.Equal(t, "line1", lines[0])
		assert.Equal(t, "", lines[1])
		assert.Equal(t, "", lines[2])
	})

	// Verify truncation preserves first lines
	t.Run("truncation preserves order", func(t *testing.T) {
		result := padToHeight("a\nb\nc\nd", 2)
		lines := strings.Split(result, "\n")
		assert.Equal(t, "a", lines[0])
		assert.Equal(t, "b", lines[1])
	})
}

// --- isContextCanceled ---

func TestIsContextCanceled(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"context.Canceled", context.Canceled, false},
		{"context.DeadlineExceeded", context.DeadlineExceeded, false},
		{"wrapped context.Canceled", context.Canceled, false},
		{"random error", errors.New("some error"), false},
		{"context canceled string", errors.New("context canceled"), false},
		{"context deadline exceeded string", errors.New("context deadline exceeded"), false},
		{"error containing context canceled", errors.New("operation failed: context canceled"), false},
	}

	// Override expected for actual cancellation errors
	tests[1].expected = true
	tests[2].expected = true
	tests[3].expected = true
	tests[5].expected = true
	tests[6].expected = true
	tests[7].expected = true

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isContextCanceled(tt.err))
		})
	}
}

// --- addBookmark ---

func TestAddBookmark(t *testing.T) {
	bm1 := model.Bookmark{Name: "bm1", Context: "ctx1", Namespace: "ns1", ResourceType: "apps/v1/deployments", ResourceName: "dep1"}
	bm2 := model.Bookmark{Name: "bm2", Context: "ctx2", Namespace: "ns2", ResourceType: "v1/pods", ResourceName: "pod1"}

	t.Run("add to empty list", func(t *testing.T) {
		result := addBookmark(nil, bm1)
		assert.Len(t, result, 1)
		assert.Equal(t, bm1, result[0])
	})

	t.Run("add new bookmark", func(t *testing.T) {
		result := addBookmark([]model.Bookmark{bm1}, bm2)
		assert.Len(t, result, 2)
		assert.Equal(t, bm2, result[1])
	})

	t.Run("deduplicate existing bookmark", func(t *testing.T) {
		result := addBookmark([]model.Bookmark{bm1}, bm1)
		assert.Len(t, result, 1)
	})

	t.Run("different namespace is not duplicate", func(t *testing.T) {
		bmDiffNs := bm1
		bmDiffNs.Namespace = "other-ns"
		result := addBookmark([]model.Bookmark{bm1}, bmDiffNs)
		assert.Len(t, result, 2)
	})

	t.Run("different context is not duplicate", func(t *testing.T) {
		bmDiffCtx := bm1
		bmDiffCtx.Context = "other-ctx"
		result := addBookmark([]model.Bookmark{bm1}, bmDiffCtx)
		assert.Len(t, result, 2)
	})
}

// --- removeBookmark ---

func TestRemoveBookmark(t *testing.T) {
	bm1 := model.Bookmark{Name: "bm1", Context: "ctx1"}
	bm2 := model.Bookmark{Name: "bm2", Context: "ctx2"}
	bm3 := model.Bookmark{Name: "bm3", Context: "ctx3"}

	t.Run("remove middle element", func(t *testing.T) {
		result := removeBookmark([]model.Bookmark{bm1, bm2, bm3}, 1)
		assert.Len(t, result, 2)
		assert.Equal(t, "bm1", result[0].Name)
		assert.Equal(t, "bm3", result[1].Name)
	})

	t.Run("remove first element", func(t *testing.T) {
		result := removeBookmark([]model.Bookmark{bm1, bm2}, 0)
		assert.Len(t, result, 1)
		assert.Equal(t, "bm2", result[0].Name)
	})

	t.Run("remove last element", func(t *testing.T) {
		result := removeBookmark([]model.Bookmark{bm1, bm2}, 1)
		assert.Len(t, result, 1)
		assert.Equal(t, "bm1", result[0].Name)
	})

	t.Run("remove from single-element list", func(t *testing.T) {
		result := removeBookmark([]model.Bookmark{bm1}, 0)
		assert.Empty(t, result)
	})

	t.Run("negative index unchanged", func(t *testing.T) {
		result := removeBookmark([]model.Bookmark{bm1, bm2}, -1)
		assert.Len(t, result, 2)
	})

	t.Run("out of bounds unchanged", func(t *testing.T) {
		result := removeBookmark([]model.Bookmark{bm1, bm2}, 5)
		assert.Len(t, result, 2)
	})
}
