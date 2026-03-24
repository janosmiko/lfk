package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- secretValueDisplay ---

func TestSecretValueDisplay(t *testing.T) {
	tests := []struct {
		name     string
		val      string
		revealed bool
		maxW     int
		expected string
	}{
		{
			name:     "hidden value shows mask",
			val:      "super-secret",
			revealed: false,
			maxW:     20,
			expected: "********",
		},
		{
			name:     "revealed value shows actual",
			val:      "mypassword",
			revealed: true,
			maxW:     20,
			expected: "mypassword",
		},
		{
			name:     "revealed long value truncated",
			val:      "a-very-long-secret-value-that-exceeds-width",
			revealed: true,
			maxW:     15,
		},
		{
			name:     "empty revealed value",
			val:      "",
			revealed: true,
			maxW:     20,
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := secretValueDisplay(tt.val, tt.revealed, tt.maxW)
			if tt.expected != "" {
				assert.Equal(t, tt.expected, result)
			}
			if tt.revealed && tt.maxW > 0 && len(tt.val) > tt.maxW {
				// Truncated value should be shorter than the original.
				assert.LessOrEqual(t, len(result), tt.maxW)
			}
		})
	}
}

// --- renderSecretEditorTable ---

func TestRenderSecretEditorTable(t *testing.T) {
	t.Run("empty secret shows add hint", func(t *testing.T) {
		secret := &model.SecretData{
			Keys: []string{},
			Data: map[string]string{},
		}
		result := renderSecretEditorTable(secret, 0, nil, false, false, "", "", 0, 60, 20)
		assert.Contains(t, result, "Key")
		assert.Contains(t, result, "Value")
		assert.Contains(t, result, "(empty - press 'a' to add a key)")
	})

	t.Run("hidden values show mask", func(t *testing.T) {
		secret := &model.SecretData{
			Keys: []string{"password", "token"},
			Data: map[string]string{"password": "secret123", "token": "abc"},
		}
		result := renderSecretEditorTable(secret, 0, nil, false, false, "", "", 0, 80, 20)
		assert.Contains(t, result, "password")
		assert.Contains(t, result, "********")
		// The actual value should not appear when not revealed.
		assert.NotContains(t, result, "secret123")
	})

	t.Run("revealed keys show actual values", func(t *testing.T) {
		secret := &model.SecretData{
			Keys: []string{"password"},
			Data: map[string]string{"password": "secret123"},
		}
		revealed := map[string]bool{"password": true}
		result := renderSecretEditorTable(secret, 0, revealed, false, false, "", "", 0, 80, 20)
		assert.Contains(t, result, "secret123")
	})

	t.Run("allRevealed shows all values", func(t *testing.T) {
		secret := &model.SecretData{
			Keys: []string{"password", "token"},
			Data: map[string]string{"password": "pass1", "token": "tok1"},
		}
		result := renderSecretEditorTable(secret, 0, nil, true, false, "", "", 0, 80, 20)
		assert.Contains(t, result, "pass1")
		assert.Contains(t, result, "tok1")
	})

	t.Run("selected row shows cursor", func(t *testing.T) {
		secret := &model.SecretData{
			Keys: []string{"key1", "key2"},
			Data: map[string]string{"key1": "v1", "key2": "v2"},
		}
		result := renderSecretEditorTable(secret, 1, nil, false, false, "", "", 0, 60, 20)
		assert.Contains(t, result, ">")
	})

	t.Run("editing key column shows edit cursor", func(t *testing.T) {
		secret := &model.SecretData{
			Keys: []string{"mykey"},
			Data: map[string]string{"mykey": "myval"},
		}
		result := renderSecretEditorTable(secret, 0, nil, false, true, "newkey", "", 0, 60, 20)
		assert.Contains(t, result, "newkey")
		assert.Contains(t, result, "\u2588")
	})

	t.Run("editing value column shows edit cursor", func(t *testing.T) {
		secret := &model.SecretData{
			Keys: []string{"mykey"},
			Data: map[string]string{"mykey": "myval"},
		}
		result := renderSecretEditorTable(secret, 0, nil, false, true, "", "newval", 1, 60, 20)
		assert.Contains(t, result, "newval")
		assert.Contains(t, result, "\u2588")
	})
}

// --- RenderSecretEditorOverlay ---

func TestRenderSecretEditorOverlay(t *testing.T) {
	t.Run("nil secret shows error", func(t *testing.T) {
		result := RenderSecretEditorOverlay(nil, 0, nil, false, false, "", "", 0, 100, 40)
		assert.Contains(t, result, "No secret loaded")
	})

	t.Run("normal mode hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		secret := &model.SecretData{
			Keys: []string{"key1"},
			Data: map[string]string{"key1": "val1"},
		}
		result := RenderSecretEditorOverlay(secret, 0, nil, false, false, "", "", 0, 100, 40)
		assert.Contains(t, result, "Secret Editor")
		assert.Contains(t, result, "key1")
	})

	t.Run("editing mode hints removed from overlay body", func(t *testing.T) {
		// Hints now live in the main status bar, not inline.
		secret := &model.SecretData{
			Keys: []string{"key1"},
			Data: map[string]string{"key1": "val1"},
		}
		result := RenderSecretEditorOverlay(secret, 0, nil, false, true, "key1", "val1", 1, 100, 40)
		assert.Contains(t, result, "Secret Editor")
	})
}
