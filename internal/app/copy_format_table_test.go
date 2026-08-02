package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func TestBuildCopyTable_SingleRow(t *testing.T) {
	items := []model.Item{
		{Name: "nginx-abc", Namespace: "default", Ready: "1/1", Status: "Running", Restarts: "0", Age: "3d"},
	}
	got := BuildCopyTable(items, []string{"Namespace", "Name", "Ready", "Status", "Restarts", "Age"})
	want := "NAMESPACE  NAME       READY  STATUS   RESTARTS  AGE\n" +
		"default    nginx-abc  1/1    Running  0         3d\n"
	assert.Equal(t, want, got)
}

func TestBuildCopyTable_WidthFromLongestValue(t *testing.T) {
	items := []model.Item{
		{Name: "short"},
		{Name: "verylongresourcename"},
	}
	got := BuildCopyTable(items, []string{"Name"})
	want := "NAME\n" +
		"short\n" +
		"verylongresourcename\n"
	assert.Equal(t, want, got)
}

func TestBuildCopyTable_EmptyBuiltinAsNone(t *testing.T) {
	items := []model.Item{{Name: "x"}}
	got := BuildCopyTable(items, []string{"Name", "Namespace", "Ready"})
	assert.Contains(t, got, "<none>")
	// header still uppercased
	assert.True(t, strings.HasPrefix(got, "NAME"))
}

func TestBuildCopyTable_StripsSortArrows(t *testing.T) {
	items := []model.Item{{Name: "x", Status: "↑ Running"}}
	got := BuildCopyTable(items, []string{"Name", "Status"})
	assert.NotContains(t, got, "↑")
	assert.Contains(t, got, "Running")
}

func TestBuildCopyTable_StripsDownArrow(t *testing.T) {
	items := []model.Item{{Name: "x", Status: "↓ Pending"}}
	got := BuildCopyTable(items, []string{"Name", "Status"})
	assert.NotContains(t, got, "↓")
	assert.Contains(t, got, "Pending")
}

func TestBuildCopyTable_MultibyteWidthAlignment(t *testing.T) {
	items := []model.Item{
		{Name: "ascii-only", Namespace: "default"},
		{Name: "second-row", Namespace: "kube-système"},
	}
	got := BuildCopyTable(items, []string{"Namespace", "Name"})
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	// All lines should have the Name column starting at the same visual column.
	// With lipgloss.Width-based padding, this means the rune-length of the
	// prefix (Namespace cell + separator) is identical on every line.
	require.Len(t, lines, 3, "header + 2 rows")
	// Find the visual column position of the NAME header. LastIndex avoids
	// matching the NAME prefix inside the NAMESPACE header in column 0.
	headerIdx := strings.LastIndex(lines[0], "NAME")
	require.NotEqual(t, -1, headerIdx)
	// Row 1's "ascii-only" starts where? Use the rune-prefix length.
	for i, line := range lines[1:] {
		nameStart := strings.Index(line, []string{"ascii-only", "second-row"}[i])
		require.NotEqual(t, -1, nameStart, "row %d: name should appear", i)
		// Convert nameStart from byte index to visual column index via lipgloss.Width of the prefix
		prefix := line[:nameStart]
		assert.Equal(t, headerIdx, lipgloss.Width(prefix),
			"row %d: visual column of name must match header column", i)
	}
}

func TestBuildCopyTable_ExtraColumnsFromItemColumns(t *testing.T) {
	items := []model.Item{
		{
			Name:    "thing",
			Columns: []model.KeyValue{{Key: "Image", Value: "nginx:1.25"}},
		},
	}
	got := BuildCopyTable(items, []string{"Name", "Image"})
	assert.Contains(t, got, "IMAGE")
	assert.Contains(t, got, "nginx:1.25")
}

func TestBuildCopyTable_NoTruncation(t *testing.T) {
	longName := strings.Repeat("a", 200)
	items := []model.Item{{Name: longName}}
	got := BuildCopyTable(items, []string{"Name"})
	assert.Contains(t, got, longName)
}

func TestBuildCopyTable_RowCount(t *testing.T) {
	items := []model.Item{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	got := BuildCopyTable(items, []string{"Name"})
	// 1 header + 3 data rows, each terminated by \n
	assert.Equal(t, 4, strings.Count(got, "\n"))
}

func TestBuildCopyTable_EmptyItemsHeaderOnly(t *testing.T) {
	got := BuildCopyTable(nil, []string{"Name", "Namespace"})
	// Header row only, terminated by \n.
	assert.Equal(t, "NAME  NAMESPACE\n", got)
}
