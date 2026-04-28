package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// --- HighlightYAMLLine ---

func TestHighlightYAMLLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantSubstr []string
	}{
		{
			name:       "comment line",
			line:       "# this is a comment",
			wantSubstr: []string{"# this is a comment"},
		},
		{
			name:       "indented comment",
			line:       "  # indented comment",
			wantSubstr: []string{"# indented comment"},
		},
		{
			name:       "key-value pair",
			line:       "name: my-pod",
			wantSubstr: []string{"name", ":", "my-pod"},
		},
		{
			name:       "key-only with colon",
			line:       "metadata:",
			wantSubstr: []string{"metadata", ":"},
		},
		{
			name:       "indented key-value",
			line:       "  replicas: 3",
			wantSubstr: []string{"replicas", ":", "3"},
		},
		{
			name:       "list item marker",
			line:       "- item-value",
			wantSubstr: []string{"-", "item-value"},
		},
		{
			name:       "indented list item",
			line:       "  - list-entry",
			wantSubstr: []string{"-", "list-entry"},
		},
		{
			name:       "plain value line",
			line:       "just a plain value",
			wantSubstr: []string{"just a plain value"},
		},
		{
			name:       "empty line",
			line:       "",
			wantSubstr: []string{""},
		},
		{
			name:       "dash-prefixed key-value",
			line:       "- name: container-1",
			wantSubstr: []string{"name", ":", "container-1"},
		},
		{
			name:       "value with spaces in key part skips yaml key styling",
			line:       "this has spaces: value",
			wantSubstr: []string{"this has spaces: value"},
		},
		{
			name:       "fold indicator expanded key-only",
			line:       "  ▾ annotations:",
			wantSubstr: []string{"▾", "annotations", ":"},
		},
		{
			name:       "fold indicator collapsed key-only",
			line:       "  ▸ annotations:",
			wantSubstr: []string{"▸", "annotations", ":"},
		},
		{
			name:       "fold indicator with key-value",
			line:       "▾ metadata:",
			wantSubstr: []string{"▾", "metadata", ":"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HighlightYAMLLine(tt.line)
			for _, sub := range tt.wantSubstr {
				assert.Contains(t, result, sub, "result should contain %q", sub)
			}
		})
	}
}

// --- styleYAMLValue ---

func TestStyleYAMLValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		// checkFn inspects the styled output for expected ANSI styling.
		// We strip ANSI codes and verify the text is present, and verify
		// distinct styling is applied for different value types.
		wantText string
	}{
		// Null values.
		{name: "null keyword", value: " null", wantText: "null"},
		{name: "tilde null", value: " ~", wantText: "~"},
		{name: "Null capitalized", value: " Null", wantText: "Null"},
		{name: "NULL uppercase", value: " NULL", wantText: "NULL"},

		// Boolean values.
		{name: "true", value: " true", wantText: "true"},
		{name: "false", value: " false", wantText: "false"},
		{name: "True", value: " True", wantText: "True"},
		{name: "False", value: " False", wantText: "False"},
		{name: "yes", value: " yes", wantText: "yes"},
		{name: "no", value: " no", wantText: "no"},
		{name: "on", value: " on", wantText: "on"},
		{name: "off", value: " off", wantText: "off"},

		// Quoted strings.
		{name: "double-quoted string", value: ` "hello world"`, wantText: `"hello world"`},
		{name: "single-quoted string", value: " 'value'", wantText: "'value'"},

		// Numeric values.
		{name: "integer", value: " 42", wantText: "42"},
		{name: "negative integer", value: " -3", wantText: "-3"},
		{name: "float", value: " 3.14", wantText: "3.14"},
		{name: "hex number", value: " 0xFF", wantText: "0xFF"},
		{name: "octal number", value: " 0o755", wantText: "0o755"},
		{name: "infinity", value: " .inf", wantText: ".inf"},
		{name: "nan", value: " .nan", wantText: ".nan"},
		{name: "scientific notation", value: " 1e10", wantText: "1e10"},

		// Anchors & aliases.
		{name: "anchor", value: " &default", wantText: "&default"},
		{name: "alias", value: " *default", wantText: "*default"},

		// Tags.
		{name: "tag", value: " !!str", wantText: "!!str"},
		{name: "single-bang tag", value: " !custom", wantText: "!custom"},

		// Block scalar indicators.
		{name: "literal block", value: " |", wantText: "|"},
		{name: "folded block", value: " >", wantText: ">"},
		{name: "literal strip", value: " |-", wantText: "|-"},
		{name: "folded strip", value: " >-", wantText: ">-"},
		{name: "literal keep", value: " |+", wantText: "|+"},
		{name: "folded keep", value: " >+", wantText: ">+"},

		// Plain strings (no special styling).
		{name: "plain string", value: " my-pod", wantText: "my-pod"},
		{name: "empty", value: "", wantText: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := styleYAMLValue(tt.value)
			assert.Contains(t, result, tt.wantText,
				"styled result should contain the original text %q", tt.wantText)
		})
	}
}

// --- styleYAMLValue distinct styles ---

func TestStyleYAMLValueDistinctStyles(t *testing.T) {
	// Different value types should produce different styled output compared
	// to a plain unquoted string.
	plain := styleYAMLValue(" my-string")
	specials := []string{
		" true",    // bool
		" null",    // null
		" 42",      // number
		" &anchor", // anchor
		" |",       // block scalar
	}
	for _, s := range specials {
		styled := styleYAMLValue(s)
		assert.NotEqual(t, styled, plain,
			"style(%q) and style(%q) should differ", s, " my-string")
	}

	// Quoted and unquoted strings should use the same string color.
	quoted := styleYAMLValue(` "hello"`)
	unquoted := styleYAMLValue(" hello")
	assert.Contains(t, quoted, "hello")
	assert.Contains(t, unquoted, "hello")
}

// --- findInlineComment ---

func TestFindInlineComment(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{name: "no comment", s: "value", want: -1},
		{name: "inline comment", s: "value # comment", want: 5},
		{name: "hash without space before", s: "val#ue", want: -1},
		{name: "hash in double quotes", s: `"val # ue" # real`, want: 10},
		{name: "hash in single quotes", s: "'val # ue' # real", want: 10},
		{name: "no hash at all", s: "plain value", want: -1},
		{name: "comment at start", s: " # comment", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findInlineComment(tt.s)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- findYAMLColon ---

func TestFindYAMLColon(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{name: "simple key", s: "name: value", want: 4},
		{name: "key only", s: "metadata:", want: 8},
		{name: "no colon", s: "just text", want: -1},
		{name: "colon in URL no space", s: "http://example.com", want: -1},
		{name: "colon in quoted key", s: `"key:name": value`, want: 10},
		{name: "colon mid-word", s: "host:8080", want: -1},
		{name: "dotted key", s: "app.kubernetes.io/name: val", want: 22},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findYAMLColon(tt.s)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- isYAMLKey ---

func TestIsYAMLKey(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{name: "simple", s: "name", want: true},
		{name: "dotted", s: "app.kubernetes.io/name", want: true},
		{name: "with-dash", s: "my-key", want: true},
		{name: "double-quoted", s: `"my key"`, want: true},
		{name: "single-quoted", s: "'my key'", want: true},
		{name: "has spaces", s: "not a key", want: false},
		{name: "empty", s: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isYAMLKey(tt.s)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- HighlightYAMLLine with value-type awareness ---

func TestHighlightYAMLLine_ValueTypes(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantSubstr []string
	}{
		{
			name:       "boolean value",
			line:       "enabled: true",
			wantSubstr: []string{"enabled", ":", "true"},
		},
		{
			name:       "null value",
			line:       "value: null",
			wantSubstr: []string{"value", ":", "null"},
		},
		{
			name:       "numeric value",
			line:       "replicas: 3",
			wantSubstr: []string{"replicas", ":", "3"},
		},
		{
			name:       "quoted string value",
			line:       `name: "my-pod"`,
			wantSubstr: []string{"name", ":", `"my-pod"`},
		},
		{
			name:       "block scalar indicator",
			line:       "script: |",
			wantSubstr: []string{"script", ":", "|"},
		},
		{
			name:       "list item with key-value",
			line:       "  - name: container-1",
			wantSubstr: []string{"-", "name", ":", "container-1"},
		},
		{
			name:       "list item with boolean",
			line:       "  - enabled: false",
			wantSubstr: []string{"-", "enabled", ":", "false"},
		},
		{
			name:       "inline comment",
			line:       "port: 8080 # HTTP port",
			wantSubstr: []string{"port", ":", "8080", "# HTTP port"},
		},
		{
			name:       "URL value not split on internal colon",
			line:       "url: http://example.com:8080",
			wantSubstr: []string{"url", ":", "http://example.com:8080"},
		},
		{
			name:       "dotted key",
			line:       "app.kubernetes.io/name: my-app",
			wantSubstr: []string{"app.kubernetes.io/name", ":", "my-app"},
		},
		{
			name:       "quoted key",
			line:       `"my key": value`,
			wantSubstr: []string{"my key", ":", "value"},
		},
		{
			name:       "fold indicator section key",
			line:       "  ▾ annotations:",
			wantSubstr: []string{"▾", "annotations", ":"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HighlightYAMLLine(tt.line)
			for _, sub := range tt.wantSubstr {
				assert.Contains(t, result, sub,
					"result should contain %q", sub)
			}
		})
	}
}

// TestHighlightYAMLLine_FoldKeySameStyleAsPlainKey verifies that fold-indicated
// section keys use the same key color as regular keys.
func TestHighlightYAMLLine_FoldKeySameStyleAsPlainKey(t *testing.T) {
	// "name" in "name: value" and "annotations" in "▾ annotations:" should
	// both be rendered with YamlKeyStyle.
	plainResult := HighlightYAMLLine("name: value")
	foldResult := HighlightYAMLLine("▾ annotations:")

	// Extract the styled key portion. YamlKeyStyle renders with the same ANSI
	// escape prefix regardless of text. Verify both contain that prefix.
	plainKeyStyled := YamlKeyStyle.Render("name")
	foldKeyStyled := YamlKeyStyle.Render("annotations")

	assert.Contains(t, plainResult, plainKeyStyled,
		"plain key should use YamlKeyStyle")
	assert.Contains(t, foldResult, foldKeyStyled,
		"fold-indicated key should use YamlKeyStyle")
}

// --- HighlightSearchInLine ---

func TestHighlightSearchInLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		query      string
		isCurrent  bool
		wantSubstr []string
		wantAbsent []string
	}{
		{
			name:       "empty query returns YAML-highlighted line",
			line:       "name: test",
			query:      "",
			isCurrent:  false,
			wantSubstr: []string{"name", "test"},
		},
		{
			name:       "no match returns YAML-highlighted line",
			line:       "name: test",
			query:      "xyz",
			isCurrent:  false,
			wantSubstr: []string{"name", "test"},
		},
		{
			name:       "case-insensitive match highlights query",
			line:       "Name: MyPod",
			query:      "mypod",
			isCurrent:  false,
			wantSubstr: []string{"MyPod"},
		},
		{
			name:       "match in middle of line",
			line:       "the target value here",
			query:      "target",
			isCurrent:  false,
			wantSubstr: []string{"target"},
		},
		{
			name:       "current match uses different style",
			line:       "search me here",
			query:      "me",
			isCurrent:  true,
			wantSubstr: []string{"me"},
		},
		{
			name:       "multiple matches highlighted",
			line:       "foo bar foo baz foo",
			query:      "foo",
			isCurrent:  false,
			wantSubstr: []string{"foo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HighlightSearchInLine(tt.line, tt.query, tt.isCurrent)
			for _, sub := range tt.wantSubstr {
				assert.Contains(t, result, sub, "result should contain %q", sub)
			}
			for _, absent := range tt.wantAbsent {
				assert.NotContains(t, result, absent, "result should not contain %q", absent)
			}
		})
	}
}

// Regression: when the search query matched the middle of a YAML token,
// the highlight wrapper's trailing reset cancelled the token's open SGR
// and the rest of the word dropped to terminal default. The user saw
// "ngi" highlighted in yellow inside "nginx" but "nx" rendered as plain
// text instead of staying in the value style.
//
// HighlightMatchInline re-emits the active SGR after each highlight reset
// so the post-match segment keeps the token's color.
func TestHighlightSearchInLine_PostMatchKeepsTokenStyle(t *testing.T) {
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	t.Cleanup(func() { lipgloss.DefaultRenderer().SetColorProfile(originalProfile) })
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI)

	// "name: nginx" — match "ngi" inside the value token "nginx".
	line := "  name: nginx"
	result := HighlightSearchInLine(line, "ngi", false)

	// Without the inline re-assertion the post-match tail "nx" would be
	// preceded directly by the highlight's "\x1b[0m" reset, leaving it
	// in terminal default color. With the fix, "nx" must be preceded by
	// a non-reset SGR open code so the value-style color carries over.
	assert.NotContains(t, result, "\x1b[0mnx",
		"trailing 'nx' must NOT come right after a bare reset — that's the regression. got %q", result)

	// And the highlight itself is on the matched substring. Build the
	// expected wrapper inline since SearchHighlightStyle's exact bytes
	// depend on the renderer profile.
	highlightOpen := styleOpenCodes(SearchHighlightStyle)
	require.NotEmpty(t, highlightOpen)
	assert.Contains(t, result, highlightOpen+"ngi",
		"matched substring must be wrapped with the search highlight bg")
}

// Regression: searching for a substring in the YAML preview used to drop
// syntax highlighting on matched lines entirely. HighlightSearchInLine
// returned the bare HighlightMatchStyled output, never running the YAML
// styler — so the matched line went from "key in yellow, value in green,
// punctuation dim" to plain text with just the search bg on the match.
//
// Now matched lines keep their YAML token styling (the open codes the
// renderer would have applied) and the search highlight overlays on top.
func TestHighlightSearchInLine_PreservesYAMLSyntaxStyling(t *testing.T) {
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	t.Cleanup(func() { lipgloss.DefaultRenderer().SetColorProfile(originalProfile) })
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI)

	line := "  name: nginx"
	withMatch := HighlightSearchInLine(line, "nginx", false)
	noMatch := HighlightSearchInLine(line, "", false)

	// Whatever ANSI codes the YAML highlighter emits for the unmatched
	// line — for the key, the punctuation, the value — must still be
	// present after we add the search overlay. styleOpenCodes pulls a
	// stable representation from a known YAML style.
	keyOpen := styleOpenCodes(YamlKeyStyle)
	require.NotEmpty(t, keyOpen, "YamlKeyStyle must emit codes for this assertion to mean anything")
	assert.Contains(t, noMatch, keyOpen,
		"baseline: no-match path must apply YAML key styling")
	assert.Contains(t, withMatch, keyOpen,
		"matched line must STILL apply YAML key styling alongside the search highlight — that was the bug")

	// And the search highlight itself must still be on the result.
	highlightOpen := styleOpenCodes(SearchHighlightStyle)
	require.NotEmpty(t, highlightOpen)
	assert.Contains(t, withMatch, highlightOpen,
		"matched line must contain the search highlight bg")
}

// --- FormatItemNameOnly ---

func TestFormatItemNameOnly(t *testing.T) {
	// Save and restore global state that affects icon rendering.
	origIconMode := IconMode
	defer func() { IconMode = origIconMode }()
	IconMode = "none"

	tests := []struct {
		name       string
		item       model.Item
		width      int
		wantSubstr []string
		wantAbsent []string
	}{
		{
			name:       "simple name",
			item:       model.Item{Name: "my-pod"},
			width:      40,
			wantSubstr: []string{"my-pod"},
		},
		{
			name:       "name with namespace",
			item:       model.Item{Name: "my-pod", Namespace: "default"},
			width:      40,
			wantSubstr: []string{"default/my-pod"},
		},
		{
			name:       "current status shows asterisk marker",
			item:       model.Item{Name: "my-context", Status: "current"},
			width:      40,
			wantSubstr: []string{"*", "my-context"},
		},
		{
			name:       "deprecated item shows warning marker",
			item:       model.Item{Name: "old-api", Deprecated: true},
			width:      40,
			wantSubstr: []string{"old-api"},
		},
		{
			name:       "truncation when name exceeds width",
			item:       model.Item{Name: "very-long-resource-name-that-exceeds"},
			width:      15,
			wantSubstr: []string{"very-long-reso"},
		},
		{
			name:       "width of 1 still returns something",
			item:       model.Item{Name: "pod"},
			width:      1,
			wantSubstr: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatItemNameOnly(tt.item, tt.width)
			for _, sub := range tt.wantSubstr {
				assert.Contains(t, result, sub, "result should contain %q", sub)
			}
			for _, absent := range tt.wantAbsent {
				assert.NotContains(t, result, absent, "result should not contain %q", absent)
			}
		})
	}
}

// --- FormatItemNameOnlyPlain ---

func TestFormatItemNameOnlyPlain(t *testing.T) {
	origIconMode := IconMode
	defer func() { IconMode = origIconMode }()
	IconMode = "none"

	tests := []struct {
		name       string
		item       model.Item
		width      int
		wantSubstr []string
	}{
		{
			name:       "simple name plain",
			item:       model.Item{Name: "nginx"},
			width:      40,
			wantSubstr: []string{"nginx"},
		},
		{
			name:       "name with namespace plain",
			item:       model.Item{Name: "nginx", Namespace: "prod"},
			width:      40,
			wantSubstr: []string{"prod/nginx"},
		},
		{
			name:       "current status plain shows asterisk",
			item:       model.Item{Name: "my-ctx", Status: "current"},
			width:      40,
			wantSubstr: []string{"* ", "my-ctx"},
		},
		{
			name:       "deprecated plain shows warning symbol",
			item:       model.Item{Name: "old-api", Deprecated: true},
			width:      40,
			wantSubstr: []string{"old-api"},
		},
		{
			name:       "truncation plain",
			item:       model.Item{Name: "a-very-long-resource-name"},
			width:      12,
			wantSubstr: []string{"a-very-long"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatItemNameOnlyPlain(tt.item, tt.width)
			for _, sub := range tt.wantSubstr {
				assert.Contains(t, result, sub, "result should contain %q", sub)
			}
		})
	}
}

// --- FormatItemNameOnly with icons ---

func TestFormatItemNameOnly_WithIcons(t *testing.T) {
	origIconMode := IconMode
	defer func() { IconMode = origIconMode }()
	IconMode = "unicode"

	t.Run("icon is shown before name", func(t *testing.T) {
		item := model.Item{Name: "my-pod", Icon: model.Icon{Unicode: "⬤"}}
		result := FormatItemNameOnly(item, 40)
		assert.Contains(t, result, "my-pod")
		// The icon should appear before the name.
		iconIdx := strings.Index(result, "⬤")
		nameIdx := strings.Index(result, "my-pod")
		if iconIdx >= 0 && nameIdx >= 0 {
			assert.Less(t, iconIdx, nameIdx, "icon should appear before name")
		}
	})

	t.Run("icon with current status", func(t *testing.T) {
		item := model.Item{Name: "my-ctx", Icon: model.Icon{Unicode: "⬤"}, Status: "current"}
		result := FormatItemNameOnly(item, 40)
		assert.Contains(t, result, "*")
		assert.Contains(t, result, "my-ctx")
	})
}

// --- FormatItemNameOnlyPlain with icons ---

func TestFormatItemNameOnlyPlain_WithIcons(t *testing.T) {
	origIconMode := IconMode
	defer func() { IconMode = origIconMode }()
	IconMode = "unicode"

	t.Run("icon is shown before name in plain", func(t *testing.T) {
		item := model.Item{Name: "my-deploy", Icon: model.Icon{Unicode: "◆"}}
		result := FormatItemNameOnlyPlain(item, 40)
		assert.Contains(t, result, "my-deploy")
		iconIdx := strings.Index(result, "◆")
		nameIdx := strings.Index(result, "my-deploy")
		if iconIdx >= 0 && nameIdx >= 0 {
			assert.Less(t, iconIdx, nameIdx, "icon should appear before name")
		}
	})
}

// --- FormatItemNameOnly read-only ---

func TestFormatItemNameOnly_ReadOnly(t *testing.T) {
	t.Run("read-only context shows [RO] marker", func(t *testing.T) {
		item := model.Item{Name: "prod", ReadOnly: true}
		result := FormatItemNameOnly(item, 40)
		assert.Contains(t, result, "prod")
		assert.Contains(t, result, "[RO]")
	})

	t.Run("current read-only context shows star and [RO]", func(t *testing.T) {
		item := model.Item{Name: "prod", Status: "current", ReadOnly: true}
		result := FormatItemNameOnly(item, 40)
		assert.Contains(t, result, "*")
		assert.Contains(t, result, "prod")
		assert.Contains(t, result, "[RO]")
	})

	t.Run("non-read-only row does not get marker", func(t *testing.T) {
		item := model.Item{Name: "dev"}
		result := FormatItemNameOnly(item, 40)
		assert.Contains(t, result, "dev")
		assert.NotContains(t, result, "[RO]")
	})
}

// --- FormatItemNameOnlyPlain read-only ---

func TestFormatItemNameOnlyPlain_ReadOnly(t *testing.T) {
	t.Run("read-only context shows [RO] marker in plain", func(t *testing.T) {
		item := model.Item{Name: "prod", ReadOnly: true}
		result := FormatItemNameOnlyPlain(item, 40)
		assert.Contains(t, result, "prod")
		assert.Contains(t, result, "[RO]")
	})

	t.Run("current read-only context shows star and [RO] in plain", func(t *testing.T) {
		item := model.Item{Name: "prod", Status: "current", ReadOnly: true}
		result := FormatItemNameOnlyPlain(item, 40)
		assert.Contains(t, result, "*")
		assert.Contains(t, result, "prod")
		assert.Contains(t, result, "[RO]")
	})

	t.Run("non-read-only row does not get marker", func(t *testing.T) {
		item := model.Item{Name: "dev"}
		result := FormatItemNameOnlyPlain(item, 40)
		assert.Contains(t, result, "dev")
		assert.NotContains(t, result, "[RO]")
	})
}

// --- wrapExtraValue ---

func TestWrapExtraValue(t *testing.T) {
	tests := []struct {
		name     string
		val      string
		width    int
		expected []string
	}{
		{
			name:     "zero width returns nil",
			val:      "hello",
			width:    0,
			expected: nil,
		},
		{
			name:     "negative width returns nil",
			val:      "hello",
			width:    -1,
			expected: nil,
		},
		{
			name:     "value fits in width returns nil",
			val:      "short",
			width:    10,
			expected: nil,
		},
		{
			name:     "value exactly at width returns nil",
			val:      "exact",
			width:    5,
			expected: nil,
		},
		{
			name:     "value needs one continuation line",
			val:      "abcdefghij",
			width:    5,
			expected: []string{"fghij"},
		},
		{
			name:     "value needs two continuation lines",
			val:      "abcdefghijklmno",
			width:    5,
			expected: []string{"fghij", "klmno"},
		},
		{
			name:     "value with partial last chunk",
			val:      "abcdefgh",
			width:    5,
			expected: []string{"fgh"},
		},
		{
			name:     "unicode runes wrap correctly",
			val:      "héllo wörld",
			width:    5,
			expected: []string{" wörl", "d"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, wrapExtraValue(tt.val, tt.width))
		})
	}
}

// --- itemExtraLines ---

func TestItemExtraLines(t *testing.T) {
	t.Run("nil item returns 0", func(t *testing.T) {
		cols := []extraColumn{{key: "MSG", width: 10}}
		assert.Equal(t, 0, itemExtraLines(nil, cols))
	})

	t.Run("no extra columns returns 0", func(t *testing.T) {
		item := &model.Item{Name: "pod"}
		assert.Equal(t, 0, itemExtraLines(item, nil))
	})

	t.Run("value fits in last column width", func(t *testing.T) {
		item := &model.Item{
			Name:    "pod",
			Columns: []model.KeyValue{{Key: "MSG", Value: "short"}},
		}
		cols := []extraColumn{{key: "MSG", width: 20}}
		assert.Equal(t, 0, itemExtraLines(item, cols))
	})

	t.Run("value needs wrapping returns 0 (wrapping disabled)", func(t *testing.T) {
		item := &model.Item{
			Name:    "pod",
			Columns: []model.KeyValue{{Key: "MSG", Value: "this is a really long message that needs wrapping"}},
		}
		// Wrapping is disabled — always returns 0 regardless of value length.
		cols := []extraColumn{{key: "MSG", width: 11}}
		assert.Equal(t, 0, itemExtraLines(item, cols))
	})

	t.Run("uses last column only", func(t *testing.T) {
		item := &model.Item{
			Name: "pod",
			Columns: []model.KeyValue{
				{Key: "IP", Value: "10.0.0.1"},
				{Key: "MSG", Value: "short"},
			},
		}
		cols := []extraColumn{
			{key: "IP", width: 5},
			{key: "MSG", width: 20},
		}
		// Only the last column (MSG) is checked.
		assert.Equal(t, 0, itemExtraLines(item, cols))
	})

	t.Run("last column width of 1 returns 0", func(t *testing.T) {
		item := &model.Item{
			Name:    "pod",
			Columns: []model.KeyValue{{Key: "MSG", Value: "hello"}},
		}
		// width=1 means wrapWidth=0, which returns 0.
		cols := []extraColumn{{key: "MSG", width: 1}}
		assert.Equal(t, 0, itemExtraLines(item, cols))
	})
}

// --- RenderYAMLContent ---

func TestRenderYAMLContent(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		width      int
		height     int
		wantSubstr []string
	}{
		{
			name:       "simple YAML",
			content:    "name: test\nreplicas: 3",
			width:      80,
			height:     10,
			wantSubstr: []string{"name", "test", "replicas", "3"},
		},
		{
			name:       "truncated to height",
			content:    "line1: a\nline2: b\nline3: c\nline4: d",
			width:      80,
			height:     2,
			wantSubstr: []string{"line1", "line2"},
		},
		{
			name:       "empty content",
			content:    "",
			width:      80,
			height:     10,
			wantSubstr: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderYAMLContent(tt.content, tt.width, tt.height)
			for _, sub := range tt.wantSubstr {
				assert.Contains(t, result, sub, "result should contain %q", sub)
			}
		})
	}

	t.Run("truncated content does not include extra lines", func(t *testing.T) {
		content := "line1: a\nline2: b\nline3: c"
		result := RenderYAMLContent(content, 80, 2)
		assert.NotContains(t, result, "line3")
	})
}

// --- RenderContainerDetail ---

func TestRenderContainerDetail(t *testing.T) {
	t.Run("basic container", func(t *testing.T) {
		item := &model.Item{
			Name:   "nginx",
			Status: "Running",
			Extra:  "nginx:1.25",
		}
		result := RenderContainerDetail(item, 80, 20)
		assert.Contains(t, result, "CONTAINER DETAILS")
		assert.Contains(t, result, "nginx")
		assert.Contains(t, result, "Running")
		assert.Contains(t, result, "nginx:1.25")
	})

	t.Run("init container type shown", func(t *testing.T) {
		item := &model.Item{
			Name:     "init-db",
			Status:   "Completed",
			Category: "Init Containers",
		}
		result := RenderContainerDetail(item, 80, 20)
		assert.Contains(t, result, "Init Container")
	})

	t.Run("sidecar container type shown", func(t *testing.T) {
		item := &model.Item{
			Name:     "envoy",
			Status:   "Running",
			Category: "Sidecar Containers",
		}
		result := RenderContainerDetail(item, 80, 20)
		assert.Contains(t, result, "Sidecar Container")
	})

	t.Run("ready and restarts shown", func(t *testing.T) {
		item := &model.Item{
			Name:     "app",
			Status:   "Running",
			Ready:    "1/1",
			Restarts: "3",
			Age:      "5m",
		}
		result := RenderContainerDetail(item, 80, 20)
		assert.Contains(t, result, "1/1")
		assert.Contains(t, result, "3")
		assert.Contains(t, result, "5m")
	})

	t.Run("additional columns displayed", func(t *testing.T) {
		item := &model.Item{
			Name:   "app",
			Status: "Running",
			Columns: []model.KeyValue{
				{Key: "Port", Value: "8080"},
				{Key: "Protocol", Value: "TCP"},
			},
		}
		result := RenderContainerDetail(item, 80, 20)
		assert.Contains(t, result, "Port")
		assert.Contains(t, result, "8080")
		assert.Contains(t, result, "Protocol")
		assert.Contains(t, result, "TCP")
	})

	t.Run("skips internal columns", func(t *testing.T) {
		item := &model.Item{
			Name:   "app",
			Status: "Running",
			Columns: []model.KeyValue{
				{Key: "__internal", Value: "hidden"},
				{Key: "owner:deploy", Value: "hidden"},
				{Key: "secret:key", Value: "hidden"},
				{Key: "data:field", Value: "hidden"},
				{Key: "Visible", Value: "shown"},
			},
		}
		result := RenderContainerDetail(item, 80, 20)
		assert.NotContains(t, result, "hidden")
		assert.Contains(t, result, "Visible")
		assert.Contains(t, result, "shown")
	})

	t.Run("height limits output", func(t *testing.T) {
		item := &model.Item{
			Name:     "app",
			Status:   "Running",
			Extra:    "image:latest",
			Ready:    "1/1",
			Restarts: "0",
			Age:      "1d",
		}
		// Height of 5 should truncate (header=2 + 3 rows).
		result := RenderContainerDetail(item, 80, 5)
		lines := strings.Split(result, "\n")
		assert.LessOrEqual(t, len(lines), 5)
	})
}
