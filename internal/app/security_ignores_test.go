package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddSecurityIgnoreGlobal(t *testing.T) {
	original := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}

	rule := SecurityIgnoreRule{Source: "heuristic", GroupKey: "CVE-2024-1234"}
	updated := addSecurityIgnore(original, "prod", rule)

	// Verify rule was added.
	require.Len(t, updated.Contexts["prod"], 1)
	assert.Equal(t, "CVE-2024-1234", updated.Contexts["prod"][0].GroupKey)
	assert.Empty(t, updated.Contexts["prod"][0].Resource)
	assert.NotEmpty(t, updated.Contexts["prod"][0].CreatedAt)

	// Verify original is unchanged (immutability).
	assert.Empty(t, original.Contexts["prod"])
}

func TestAddSecurityIgnoreResource(t *testing.T) {
	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}

	rule := SecurityIgnoreRule{
		Source:   "heuristic",
		GroupKey: "CVE-2024-5678",
		Resource: "default/Deployment/nginx",
	}
	updated := addSecurityIgnore(state, "staging", rule)

	require.Len(t, updated.Contexts["staging"], 1)
	assert.Equal(t, "CVE-2024-5678", updated.Contexts["staging"][0].GroupKey)
	assert.Equal(t, "default/Deployment/nginx", updated.Contexts["staging"][0].Resource)
}

func TestAddSecurityIgnoreDeduplicates(t *testing.T) {
	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}

	rule := SecurityIgnoreRule{
		Source:   "heuristic",
		GroupKey: "CVE-2024-1234",
		Resource: "default/Pod/app",
		Comment:  "first",
	}
	state = addSecurityIgnore(state, "prod", rule)

	// Add same (GroupKey, Resource) again with a different comment.
	rule2 := SecurityIgnoreRule{
		Source:   "heuristic",
		GroupKey: "CVE-2024-1234",
		Resource: "default/Pod/app",
		Comment:  "updated",
	}
	state = addSecurityIgnore(state, "prod", rule2)

	// Only one entry should exist.
	require.Len(t, state.Contexts["prod"], 1)
	assert.Equal(t, "updated", state.Contexts["prod"][0].Comment)
}

func TestRemoveSecurityIgnore(t *testing.T) {
	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}

	rule := SecurityIgnoreRule{Source: "heuristic", GroupKey: "CVE-2024-1234"}
	state = addSecurityIgnore(state, "prod", rule)
	require.Len(t, state.Contexts["prod"], 1)

	state = removeSecurityIgnore(state, "prod", "heuristic", "CVE-2024-1234", "")
	assert.Empty(t, state.Contexts["prod"])
}

func TestRemoveSecurityIgnoreNonexistent(t *testing.T) {
	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}

	// Should not panic on empty state.
	result := removeSecurityIgnore(state, "prod", "heuristic", "CVE-NONE", "")
	assert.Empty(t, result.Contexts["prod"])
}

func TestIsGroupIgnored(t *testing.T) {
	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}

	// Add a global ignore (empty Resource).
	rule := SecurityIgnoreRule{Source: "heuristic", GroupKey: "CVE-2024-1234"}
	state = addSecurityIgnore(state, "prod", rule)

	assert.True(t, isGroupIgnored(state, "prod", "heuristic", "CVE-2024-1234"))
	assert.False(t, isGroupIgnored(state, "prod", "heuristic", "CVE-OTHER"))
	assert.False(t, isGroupIgnored(state, "staging", "heuristic", "CVE-2024-1234"))
}

func TestIsResourceIgnored(t *testing.T) {
	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}

	// Add a global ignore -- should match any resource of that group.
	globalRule := SecurityIgnoreRule{Source: "heuristic", GroupKey: "CVE-2024-1234"}
	state = addSecurityIgnore(state, "prod", globalRule)

	assert.True(t, isResourceIgnored(state, "prod", "heuristic", "CVE-2024-1234", "default/Pod/app"))
	assert.True(t, isResourceIgnored(state, "prod", "heuristic", "CVE-2024-1234", "kube-system/Deployment/dns"))
	assert.False(t, isResourceIgnored(state, "prod", "heuristic", "CVE-OTHER", "default/Pod/app"))

	// Add a resource-specific ignore for a different group.
	specificRule := SecurityIgnoreRule{
		Source:   "heuristic",
		GroupKey: "CVE-2024-5678",
		Resource: "default/Pod/web",
	}
	state = addSecurityIgnore(state, "prod", specificRule)

	assert.True(t, isResourceIgnored(state, "prod", "heuristic", "CVE-2024-5678", "default/Pod/web"))
	assert.False(t, isResourceIgnored(state, "prod", "heuristic", "CVE-2024-5678", "default/Pod/api"))
}

func TestIsResourceIgnoredSpecificOnly(t *testing.T) {
	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}

	// Add only a resource-specific ignore (not global).
	rule := SecurityIgnoreRule{
		Source:   "heuristic",
		GroupKey: "CVE-2024-9999",
		Resource: "ns1/Deployment/frontend",
	}
	state = addSecurityIgnore(state, "prod", rule)

	// isGroupIgnored should return false (no global ignore).
	assert.False(t, isGroupIgnored(state, "prod", "heuristic", "CVE-2024-9999"))

	// isResourceIgnored should return true only for the matching resource.
	assert.True(t, isResourceIgnored(state, "prod", "heuristic", "CVE-2024-9999", "ns1/Deployment/frontend"))
	assert.False(t, isResourceIgnored(state, "prod", "heuristic", "CVE-2024-9999", "ns1/Deployment/backend"))
}

func TestCountIgnoredGroups(t *testing.T) {
	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}

	// Add 2 global ignores.
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{Source: "heuristic", GroupKey: "CVE-A"})
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{Source: "heuristic", GroupKey: "CVE-B"})

	// Add 1 resource-specific ignore (should NOT count).
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{
		Source:   "heuristic",
		GroupKey: "CVE-C",
		Resource: "default/Pod/app",
	})

	assert.Equal(t, 2, countIgnoredGroups(state, "prod"))
	assert.Equal(t, 0, countIgnoredGroups(state, "staging"))
}

func TestSecurityIgnoresRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	state := &SecurityIgnoreState{Contexts: make(map[string][]SecurityIgnoreRule)}
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{
		Source:   "heuristic",
		GroupKey: "CVE-2024-1234",
		Comment:  "known false positive",
	})
	state = addSecurityIgnore(state, "prod", SecurityIgnoreRule{
		Source:   "heuristic",
		GroupKey: "CVE-2024-5678",
		Resource: "default/Pod/app",
	})
	state = addSecurityIgnore(state, "staging", SecurityIgnoreRule{
		Source:   "heuristic",
		GroupKey: "CVE-2024-9999",
	})

	// Save.
	err := saveSecurityIgnores(state)
	require.NoError(t, err)

	// Verify file was created.
	expectedPath := filepath.Join(tmpDir, "lfk", "security_ignores.yaml")
	_, err = os.Stat(expectedPath)
	require.NoError(t, err)

	// Load and verify.
	loaded := loadSecurityIgnores()
	require.Len(t, loaded.Contexts["prod"], 2)
	require.Len(t, loaded.Contexts["staging"], 1)

	assert.Equal(t, "CVE-2024-1234", loaded.Contexts["prod"][0].GroupKey)
	assert.Equal(t, "known false positive", loaded.Contexts["prod"][0].Comment)
	assert.Empty(t, loaded.Contexts["prod"][0].Resource)

	assert.Equal(t, "CVE-2024-5678", loaded.Contexts["prod"][1].GroupKey)
	assert.Equal(t, "default/Pod/app", loaded.Contexts["prod"][1].Resource)

	assert.Equal(t, "CVE-2024-9999", loaded.Contexts["staging"][0].GroupKey)
}
