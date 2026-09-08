package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// punctStyled reports whether the styled output renders want with the same
// escape sequences YamlPunctuationStyle produces for it.
func punctStyled(t *testing.T, styled, want string) {
	t.Helper()
	assert.Contains(t, styled, YamlPunctuationStyle.Render(want),
		"%q should carry punctuation styling for %q", styled, want)
}

func TestHighlightYAMLLine_KYAMLBracketOnlyLines(t *testing.T) {
	for _, line := range []string{"{", "}", "[", "]", "},", "],", "}],", "  },", "[]", "{}"} {
		t.Run(line, func(t *testing.T) {
			got := HighlightYAMLLine(line)
			assert.Equal(t, line, stripANSI(got), "text must survive highlighting")
			for _, r := range strings.TrimSpace(line) {
				punctStyled(t, got, string(r))
			}
		})
	}
}

func TestHighlightYAMLLine_KYAMLOpenBrace(t *testing.T) {
	got := HighlightYAMLLine("metadata: {")
	assert.Equal(t, "metadata: {", stripANSI(got))
	assert.Contains(t, got, YamlKeyStyle.Render("metadata"), "the key keeps its style")
	punctStyled(t, got, "{")
}

func TestHighlightYAMLLine_KYAMLQuotedValueWithTrailingComma(t *testing.T) {
	got := HighlightYAMLLine(`  name: "web",`)
	assert.Equal(t, `  name: "web",`, stripANSI(got))
	assert.Contains(t, got, YamlKeyStyle.Render("name"), "the key keeps its style")
	assert.Contains(t, got, YamlStringStyle.Render(`"web"`),
		"a trailing comma must not stop the value reading as a quoted string")
	punctStyled(t, got, ",")
}

func TestHighlightYAMLLine_KYAMLNumberWithTrailingComma(t *testing.T) {
	got := HighlightYAMLLine("  replicas: 3,")
	assert.Equal(t, "  replicas: 3,", stripANSI(got))
	assert.Contains(t, got, YamlNumberStyle.Render("3"), "numbers stay numbers before a comma")
	punctStyled(t, got, ",")
}

func TestHighlightYAMLLine_KYAMLFlowSequence(t *testing.T) {
	got := HighlightYAMLLine(`  args: ["a", "b"],`)
	assert.Equal(t, `  args: ["a", "b"],`, stripANSI(got))
	assert.Contains(t, got, YamlStringStyle.Render(`"a"`))
	assert.Contains(t, got, YamlStringStyle.Render(`"b"`))
	punctStyled(t, got, "[")
	punctStyled(t, got, "]")
}

func TestHighlightYAMLLine_KYAMLCuddledListOfObjects(t *testing.T) {
	got := HighlightYAMLLine("    containers: [{")
	assert.Equal(t, "    containers: [{", stripANSI(got))
	assert.Contains(t, got, YamlKeyStyle.Render("containers"))
	punctStyled(t, got, "[")
	punctStyled(t, got, "{")
}

// A comma inside a quoted KYAML string is data, not flow punctuation.
func TestHighlightYAMLLine_KYAMLCommaInsideQuotesIsNotPunctuation(t *testing.T) {
	got := HighlightYAMLLine(`  note: "a, b",`)
	assert.Equal(t, `  note: "a, b",`, stripANSI(got))
	assert.Contains(t, got, YamlStringStyle.Render(`"a, b"`),
		"the quoted value stays one string despite the comma inside it")
}

// Block YAML must render exactly as it did before KYAML support landed.
func TestHighlightYAMLLine_BlockYAMLUnchangedByKYAMLSupport(t *testing.T) {
	for _, line := range []string{
		"name: value",
		"  replicas: 3",
		"- item",
		"# a comment",
		"  description: a, b, c",
		"  empty:",
		`  quoted: "hello world"`,
	} {
		t.Run(line, func(t *testing.T) {
			assert.Equal(t, line, stripANSI(HighlightYAMLLine(line)))
		})
	}
}

func TestHighlightYAMLLine_PlainCommaListKeepsSingleStringStyle(t *testing.T) {
	got := HighlightYAMLLine("  description: a, b, c")
	assert.Contains(t, got, YamlStringStyle.Render("a, b, c"),
		"an unquoted block-YAML scalar containing commas is not flow syntax")
}
