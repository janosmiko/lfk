package model_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

// An emoji whose base codepoint is one column wide becomes two with the
// variation selector U+FE0F. Terminals disagree about honouring that: lipgloss,
// which lfk lays out with, always counts the pair as two columns, while a
// terminal that has not announced grapheme clustering draws one. The row then
// ends a column short and the panel border lands early (issue #604).
//
// lfk resolves this at render time by dropping the selector when the terminal
// measures with wcwidth, so both measures agree. This guard pins the property
// that makes that work: with the selector gone, the two width methods must
// return the same number for every icon in the tables. An icon that still
// disagrees would drift on some terminal no matter what the renderer does.
const variationSelector16 = "\ufe0f"

func TestEmojiIconWidthsAgreeOnceTheSelectorIsDropped(t *testing.T) {
	var disagree []string

	for _, g := range collectEmojiIcons(t) {
		bare := strings.ReplaceAll(g.glyph, variationSelector16, "")
		wc := ansi.WcWidth.StringWidth(bare)
		grapheme := ansi.GraphemeWidth.StringWidth(bare)
		if wc != grapheme {
			disagree = append(disagree,
				g.String()+" wcwidth="+strconv.Itoa(wc)+" graphemewidth="+strconv.Itoa(grapheme))
		}
	}

	sort.Strings(disagree)
	if len(disagree) > 0 {
		t.Errorf("these icons measure differently under the two width methods even without"+
			" U+FE0F, so no render-time normalization can keep them aligned:\n%s",
			strings.Join(disagree, "\n"))
	}
}

type emojiIcon struct {
	file  string
	line  int
	glyph string
}

func (e emojiIcon) String() string {
	cps := make([]string, 0, utf8.RuneCountInString(e.glyph))
	for _, r := range e.glyph {
		cps = append(cps, "U+"+strings.ToUpper(strconv.FormatInt(int64(r), 16)))
	}
	return e.file + ":" + strconv.Itoa(e.line) + " " + e.glyph + " (" + strings.Join(cps, " ") + ")"
}

// collectEmojiIcons returns every Emoji field value assigned in a composite
// literal anywhere in the module's production code, so a new icon table is
// covered without anyone remembering to register it here.
func collectEmojiIcons(t *testing.T) []emojiIcon {
	t.Helper()
	root := moduleRootForIcons(t)

	var out []emojiIcon
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", ".git", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		ast.Inspect(f, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "Emoji" {
				return true
			}
			lit, ok := kv.Value.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			glyph, unquoteErr := strconv.Unquote(lit.Value)
			if unquoteErr != nil || glyph == "" {
				return true
			}
			out = append(out, emojiIcon{file: rel, line: fset.Position(lit.Pos()).Line, glyph: glyph})
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking module root: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("no emoji icons found; the walk or the field name is wrong")
	}
	return out
}

func moduleRootForIcons(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate this test file")
	}
	// internal/model/icons_width_test.go -> module root
	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
}
