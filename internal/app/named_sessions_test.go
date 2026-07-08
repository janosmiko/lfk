package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeSessionName(t *testing.T) {
	assert.Equal(t, "prod-debug", sanitizeSessionName("prod-debug"))
	assert.Equal(t, "prod-debug", sanitizeSessionName("  prod debug  "))
	assert.Equal(t, "a-b-c", sanitizeSessionName("a/b\\c"))
	assert.Equal(t, "prod", sanitizeSessionName("prod!!!"))
	assert.Equal(t, "", sanitizeSessionName("   "))
	assert.Equal(t, "", sanitizeSessionName("///"))
	assert.LessOrEqual(t, len(sanitizeSessionName(string(make([]byte, 200)))), 64)
}

func TestNamedSessionRoundTripAndList(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	base := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	require.NoError(t, saveNamedSession(NamedSession{
		Name: "older", SavedAt: base,
		State: SessionState{Context: "c1", Tabs: []SessionTab{{Context: "c1"}}},
	}))
	require.NoError(t, saveNamedSession(NamedSession{
		Name: "newer", SavedAt: base.Add(time.Hour),
		State: SessionState{Context: "c2", Tabs: []SessionTab{{Context: "c2"}, {Context: "c2"}}},
	}))

	got, err := loadNamedSession("older")
	require.NoError(t, err)
	assert.Equal(t, "older", got.Name)
	assert.Equal(t, "c1", got.State.Context)

	list := listNamedSessions()
	require.Len(t, list, 2)
	assert.Equal(t, "newer", list[0].Name, "sorted by SavedAt desc")
	assert.Equal(t, "older", list[1].Name)

	existed, err := deleteNamedSession("older")
	require.NoError(t, err)
	assert.True(t, existed)
	assert.Len(t, listNamedSessions(), 1)

	existed, err = deleteNamedSession("missing")
	require.NoError(t, err)
	assert.False(t, existed)
}

func TestSaveNamedSession_RejectsDifferentNameCollision(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	base := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)

	require.NoError(t, saveNamedSession(NamedSession{
		Name: "prod debug", SavedAt: base,
		State: SessionState{Context: "c1", Tabs: []SessionTab{{Context: "c1"}}},
	}))

	// "prod/debug" sanitizes to the same "prod-debug.yaml" file as "prod debug".
	err := saveNamedSession(NamedSession{
		Name: "prod/debug", SavedAt: base.Add(time.Hour),
		State: SessionState{Context: "c2", Tabs: []SessionTab{{Context: "c2"}}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prod-debug")

	got, loadErr := loadNamedSession("prod debug")
	require.NoError(t, loadErr)
	assert.Equal(t, "prod debug", got.Name, "first session must not be overwritten")
	assert.Equal(t, "c1", got.State.Context)

	// Re-saving under the SAME name is an intended update and must still overwrite.
	require.NoError(t, saveNamedSession(NamedSession{
		Name: "prod debug", SavedAt: base.Add(2 * time.Hour),
		State: SessionState{Context: "c3", Tabs: []SessionTab{{Context: "c3"}}},
	}))
	got, loadErr = loadNamedSession("prod debug")
	require.NoError(t, loadErr)
	assert.Equal(t, "c3", got.State.Context, "same-name save should overwrite")
}

func TestFormatSavedAgo(t *testing.T) {
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, "just now", formatSavedAgo(now.Add(-10*time.Second), now))
	assert.Equal(t, "5m ago", formatSavedAgo(now.Add(-5*time.Minute), now))
	assert.Equal(t, "2h ago", formatSavedAgo(now.Add(-2*time.Hour), now))
	assert.Equal(t, "3d ago", formatSavedAgo(now.Add(-72*time.Hour), now))
}
