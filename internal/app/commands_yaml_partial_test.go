package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildBulkYAMLClipboardMsg_AllSucceeded(t *testing.T) {
	docs := []string{"a: 1", "b: 2", "c: 3"}
	msg := buildBulkYAMLClipboardMsg(docs, nil, 3)
	assert.NoError(t, msg.err)
	assert.Equal(t, 3, msg.count)
	assert.Equal(t, "a: 1\n---\nb: 2\n---\nc: 3\n", msg.content)
}

func TestBuildBulkYAMLClipboardMsg_AllFailed(t *testing.T) {
	failures := []string{"ns/a: forbidden", "ns/b: 404"}
	msg := buildBulkYAMLClipboardMsg(nil, failures, 2)
	require.Error(t, msg.err)
	assert.Empty(t, msg.content)
	assert.Equal(t, 0, msg.count)
	assert.Contains(t, msg.err.Error(), "all 2 fetches failed")
	assert.Contains(t, msg.err.Error(), "ns/a: forbidden")
	assert.Contains(t, msg.err.Error(), "ns/b: 404")
}

// TestBuildBulkYAMLClipboardMsg_PartialSuccess is the regression test for the
// "select N → Y → YAML produces empty clipboard when one row fails" bug. The
// successful docs must still be packed into content with the proper multi-doc
// joiner; err carries the per-item failures so updateYamlClipboard can surface
// them as a warning rather than dropping the clipboard write.
func TestBuildBulkYAMLClipboardMsg_PartialSuccess(t *testing.T) {
	docs := []string{"a: 1", "b: 2"}
	failures := []string{"ns/c: forbidden"}
	msg := buildBulkYAMLClipboardMsg(docs, failures, 3)
	require.Error(t, msg.err)
	assert.Equal(t, "a: 1\n---\nb: 2\n", msg.content,
		"partial-success content must still contain the docs that succeeded")
	assert.Equal(t, 2, msg.count, "count reflects successes, not total")
	assert.Contains(t, msg.err.Error(), "copied 2/3, 1 failed")
	assert.Contains(t, msg.err.Error(), "ns/c: forbidden")
}

// TestUpdateYamlClipboard_PartialSuccessStillCopies guards the
// updateYamlClipboard branch that must NOT short-circuit on err when the
// payload is non-empty. Prior to the fix, any err returned from the bulk
// fetch dropped the clipboard write entirely; now a partial-success msg
// (content + err both set) goes down the copy path with a "Warning:"
// status so the user gets the rows that succeeded.
func TestUpdateYamlClipboard_PartialSuccessStillCopies(t *testing.T) {
	m := baseModelCov()
	msg := yamlClipboardMsg{
		content: "a: 1\n---\nb: 2\n",
		count:   2,
		err:     assertableErr("copied 2/3, 1 failed: ns/c: forbidden"),
	}
	updated, cmd := m.updateYamlClipboard(msg)
	m2 := updated.(Model)
	require.NotNil(t, cmd, "partial success must still dispatch the clipboard write cmd")
	assert.True(t, strings.HasPrefix(m2.statusMessage, "Warning:"),
		"partial success surfaces as a Warning, not silent success")
}

// assertableErr is a tiny helper to build a deterministic error for tests
// without pulling in fmt at every call site.
type assertableErr string

func (e assertableErr) Error() string { return string(e) }
