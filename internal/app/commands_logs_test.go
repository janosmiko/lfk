package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- matchesContainerFilter ---

func TestMatchesContainerFilter(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		containers []string
		expected   bool
	}{
		{
			name:       "matching container",
			line:       "[pod/my-pod/nginx] log line here",
			containers: []string{"nginx", "sidecar"},
			expected:   true,
		},
		{
			name:       "non-matching container",
			line:       "[pod/my-pod/nginx] log line here",
			containers: []string{"sidecar"},
			expected:   false,
		},
		{
			name:       "no prefix passes through",
			line:       "plain log line without prefix",
			containers: []string{"nginx"},
			expected:   true,
		},
		{
			name:       "empty line passes through",
			line:       "",
			containers: []string{"nginx"},
			expected:   true,
		},
		{
			name:       "bracket but no closing bracket passes through",
			line:       "[incomplete prefix without close",
			containers: []string{"nginx"},
			expected:   true,
		},
		{
			name:       "bracket with no slash passes through",
			line:       "[noslash] content",
			containers: []string{"noslash"},
			expected:   true,
		},
		{
			name:       "multiple containers all match",
			line:       "[pod/my-pod/sidecar] some log",
			containers: []string{"nginx", "sidecar"},
			expected:   true,
		},
		{
			name:       "empty container filter means none match",
			line:       "[pod/my-pod/nginx] log",
			containers: []string{},
			expected:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, matchesContainerFilter(tt.line, tt.containers))
		})
	}
}

// --- sanitizeFilename ---

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple name", "pod-name", "pod-name"},
		{"with slashes", "ns/pod/container", "ns_pod_container"},
		{"with backslash", "path\\to\\file", "path_to_file"},
		{"with colons", "host:port", "host_port"},
		{"with spaces", "my pod name", "my_pod_name"},
		{"mixed special chars", "ns/pod:8080 name", "ns_pod_8080_name"},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeFilename(tt.input))
		})
	}
}

func TestCovSaveLoadedLogs(t *testing.T) {
	m := baseModelCov()
	m.logLines = []string{"line1", "line2", "line3"}
	m.actionCtx = actionContext{name: "test-pod"}
	path, err := m.saveLoadedLogs()
	assert.NoError(t, err)
	assert.Contains(t, path, "lfk-logs-test-pod")
}

func TestCovSaveAllLogs(t *testing.T) {
	m := baseModelWithFakeClient()
	m.logLines = []string{"line1", "line2"}
	m.logTitle = "Logs: my-pod"
	m = withActionCtx(m, "my-pod", "default", "Pod", model.ResourceTypeEntry{})
	cmd := m.saveAllLogs()
	assert.NotNil(t, cmd)
}

func TestCovBuildLogTitle(t *testing.T) {
	m := baseModelCov()
	m.actionCtx = actionContext{name: "my-pod", namespace: "default", context: "ctx"}
	title := m.buildLogTitle()
	assert.Contains(t, title, "Logs: default/my-pod")
}

func TestCovBuildLogTitleWithContainerFilter(t *testing.T) {
	m := baseModelCov()
	m.actionCtx = actionContext{name: "my-pod", namespace: "default", context: "ctx"}
	m.logContainers = []string{"main", "sidecar"}
	m.logSelectedContainers = []string{"main"}
	title := m.buildLogTitle()
	assert.Contains(t, title, "[main]")
}

func TestFinalMatchesContainerFilterEmptyLine(t *testing.T) {
	assert.True(t, matchesContainerFilter("", []string{"main"}))
}

func TestFinalMatchesContainerFilterNoBracket(t *testing.T) {
	assert.True(t, matchesContainerFilter("some log line", []string{"main"}))
}

func TestFinalMatchesContainerFilterNoCloseBracket(t *testing.T) {
	assert.True(t, matchesContainerFilter("[pod/name/container", []string{"main"}))
}

func TestFinalMatchesContainerFilterNoSlash(t *testing.T) {
	assert.True(t, matchesContainerFilter("[container] log", []string{"main"}))
}

func TestFinalMatchesContainerFilterMatch(t *testing.T) {
	assert.True(t, matchesContainerFilter("[pod/my-pod/main] log line", []string{"main"}))
}

func TestFinalMatchesContainerFilterNoMatch(t *testing.T) {
	assert.False(t, matchesContainerFilter("[pod/my-pod/sidecar] log line", []string{"main"}))
}

func TestFinalWaitForLogLineNilChannel(t *testing.T) {
	m := baseFinalModel()
	m.logCh = nil
	cmd := m.waitForLogLine()
	assert.Nil(t, cmd)
}

func TestFinalWaitForLogLineWithChannel(t *testing.T) {
	m := baseFinalModel()
	ch := make(chan string, 1)
	ch <- "test log line"
	m.logCh = ch
	cmd := m.waitForLogLine()
	require.NotNil(t, cmd)
	msg := cmd()
	logMsg := msg.(logLineMsg)
	assert.Equal(t, "test log line", logMsg.line)
	assert.False(t, logMsg.done)
}

func TestFinalWaitForLogLineChannelClosed(t *testing.T) {
	m := baseFinalModel()
	ch := make(chan string)
	close(ch)
	m.logCh = ch
	cmd := m.waitForLogLine()
	require.NotNil(t, cmd)
	msg := cmd()
	logMsg := msg.(logLineMsg)
	assert.True(t, logMsg.done)
}

func TestFinalSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"path/to/file", "path_to_file"},
		{"back\\slash", "back_slash"},
		{"has:colon", "has_colon"},
		{"has space", "has_space"},
		{"a/b:c d\\e", "a_b_c_d_e"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, sanitizeFilename(tt.input))
	}
}

func TestFinalSaveLoadedLogs(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx.name = "test-pod"
	m.logLines = []string{"line1", "line2", "line3"}
	path, err := m.saveLoadedLogs()
	require.NoError(t, err)
	assert.Contains(t, path, "lfk-logs-test-pod")
}

func TestFinalMaybeLoadMoreHistoryNotAtTop(t *testing.T) {
	m := baseFinalModel()
	m.logScroll = 5
	m.logHasMoreHistory = true
	cmd := m.maybeLoadMoreHistory()
	assert.Nil(t, cmd)
}

func TestFinalMaybeLoadMoreHistoryNoMore(t *testing.T) {
	m := baseFinalModel()
	m.logScroll = 0
	m.logHasMoreHistory = false
	cmd := m.maybeLoadMoreHistory()
	assert.Nil(t, cmd)
}

func TestFinalMaybeLoadMoreHistoryAlreadyLoading(t *testing.T) {
	m := baseFinalModel()
	m.logScroll = 0
	m.logHasMoreHistory = true
	m.logLoadingHistory = true
	cmd := m.maybeLoadMoreHistory()
	assert.Nil(t, cmd)
}

func TestFinalMaybeLoadMoreHistoryPrevious(t *testing.T) {
	m := baseFinalModel()
	m.logScroll = 0
	m.logHasMoreHistory = true
	m.logPrevious = true
	cmd := m.maybeLoadMoreHistory()
	assert.Nil(t, cmd)
}

// In Tail Logs mode the loaded buffer (typically 10 lines) is smaller
// than the viewport, so logScroll is pinned at 0 from startup. The
// auto-load must NOT fire just because the user moved the cursor up by
// a line — it should fire only once the cursor has reached the top of
// the loaded buffer ("Scrolling to the top auto-loads older history",
// per the docs).
func TestFinalMaybeLoadMoreHistoryCursorNotAtTop(t *testing.T) {
	m := baseFinalModel()
	m.logLines = []string{"l1", "l2", "l3", "l4", "l5", "l6", "l7", "l8", "l9", "l10"}
	m.logScroll = 0
	m.logCursor = 5 // mid-buffer — user navigated up but not to the top
	m.logHasMoreHistory = true
	cmd := m.maybeLoadMoreHistory()
	assert.Nil(t, cmd, "should not fire fetchOlderLogs when cursor is mid-buffer")
	assert.False(t, m.logLoadingHistory, "logLoadingHistory must not flip when load was skipped")
}

func TestFinalMaybeLoadMoreHistoryCursorAtTopFires(t *testing.T) {
	m := baseFinalModel()
	m.logLines = []string{"l1", "l2", "l3"}
	m.logScroll = 0
	m.logCursor = 0 // user reached the top of the loaded buffer
	m.logHasMoreHistory = true
	cmd := m.maybeLoadMoreHistory()
	assert.NotNil(t, cmd, "should fire fetchOlderLogs when cursor is at the top")
	assert.True(t, m.logLoadingHistory, "logLoadingHistory must be set to deduplicate concurrent triggers")
}
