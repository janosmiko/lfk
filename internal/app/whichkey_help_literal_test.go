package app

import (
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
)

// helpLiteralHandlerFiles maps a per-view help context to the source files that
// dispatch that view's keys. Hand-maintained on purpose: mode routing is spread
// across update.go and the overlay dispatchers, so there is no reliable way to
// derive "which file handles this view" from the AST.
//
// A context absent from this map is not checked. Two are deliberately absent:
//
//   - "Traffic Capture" dispatches on tea.KeyCode constants (tea.KeyEnter,
//     tea.KeyTab, ...) rather than on msg.String(), so a string-literal scan
//     sees none of its keys.
//   - "Exec Mode" documents ctrl+] prefix chords; the prefix is consumed before
//     the switch, so the chord strings never appear as case labels.
func helpLiteralHandlerFiles() map[string][]string {
	return map[string][]string{
		"Object Explorer":                {"objectexplorer.go"},
		"Log Top":                        {"update_logtop.go"},
		"API Explorer":                   {"update_explain.go"},
		"Error Log":                      {"update_overlays_errorlog.go"},
		"Can-I Browser":                  {"update_cani.go"},
		"Sync Wave Timeline":             {"update_overlays_sync_wave.go"},
		"Network Policy / Pod / Service": {"update_overlays_misc.go", "update_netpol_search.go"},
		"Event Timeline":                 {"update_overlays_events.go"},
		"YAML View":                      {"update_yaml.go", "update_yaml_visual.go", "update_vim.go"},
		"Describe View":                  {"update_describe.go", "update_vim.go"},
		"Diff View":                      {"update_diff.go", "update_vim.go"},
		"Log Viewer":                     {"update_logs.go", "update_vim.go"},
	}
}

// helpKeyDisplayAliases maps the capitalised display spellings the help catalog
// uses to the key string Bubble Tea actually delivers. Only exact, unambiguous
// renamings belong here — case-folding every token would let a row advertise
// "G" while the handler only matches "g".
func helpKeyDisplayAliases() map[string]string {
	return map[string]string{
		"Left": "left", "Right": "right", "Up": "up", "Down": "down",
		"Home": "home", "End": "end", "Backspace": "backspace",
		"Enter": "enter", "Esc": "esc", "Tab": "tab", "Space": "space",
	}
}

// helpLiteralExemptions names the (context, literal) pairs that are not raw key
// strings, with the reason. Keyed "Context|token"; "*|token" applies to every
// context, for notation that is context-free by nature.
func helpLiteralExemptions() map[string]string {
	return map[string]string{
		// Vim notation, not keystrokes. "gg" is two presses of the jump-top key;
		// the "123" forms are the count prefix (consumeCountPrefix, count_prefix.go)
		// applied to a motion; "viw"/"vaw" are text objects resolved by
		// innerWordRange/innerWORDRange (update_vim.go) after visual mode is armed.
		"*|gg":          "vim notation for two jump-top presses, not a key string",
		"*|123G":        "vim count prefix + G; the count is consumed by consumeCountPrefix, not matched as a key",
		"*|123y":        "vim count prefix + yank; same count-prefix path",
		"*|123<motion>": "placeholder for any motion, not a key",
		"*|viw":         "vim text object resolved by innerWordRange after v arms visual mode",
		"*|vaw":         "vim text object resolved by innerWordRange after v arms visual mode",

		// The opener, not an overlay key: the Sync Wave Timeline is reached from
		// the Application action menu (update_actions_sync_wave.go), so "W" is
		// never a case label in the overlay's own dispatcher.
		"Sync Wave Timeline|W": "opens the overlay from the Application action menu; not a key the overlay itself handles",
	}
}

// helpHandlerKeyLiterals returns every string literal the given files compare a
// key against — case labels plus == / != comparisons. Deliberately an
// over-approximation of "keys this file matches": a false positive here only
// costs the guard a detection, never a spurious failure.
func helpHandlerKeyLiterals(t *testing.T, files []string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	record := func(n ast.Node) {
		if bl, ok := n.(*ast.BasicLit); ok && bl.Kind == gotoken.STRING {
			if s, err := strconv.Unquote(bl.Value); err == nil {
				out[s] = true
			}
		}
	}
	fset := gotoken.NewFileSet()
	for _, f := range files {
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.CaseClause:
				for _, e := range v.List {
					ast.Inspect(e, func(m ast.Node) bool { record(m); return true })
				}
			case *ast.BinaryExpr:
				if v.Op == gotoken.EQL || v.Op == gotoken.NEQ {
					record(v.X)
					record(v.Y)
				}
			}
			return true
		})
	}
	return out
}

// sentinelKeybindings returns a Keybindings whose every string field holds a
// unique marker, so a rendered help key can be split into the parts that came
// from a config field and the parts that were typed as literals.
func sentinelKeybindings() ui.Keybindings {
	kb := ui.Keybindings{}
	v := reflect.ValueOf(&kb).Elem()
	for f, fv := range v.Fields() {
		if f.Type.Kind() == reflect.String {
			fv.SetString("__hl_" + f.Name + "__")
		}
	}
	return kb
}

// TestHelpSections_LiteralKeysAreReallyLiteral is the forcing function against
// the class of bug the rebind audit found four times: a help row that prints a
// hardcoded key its handler no longer matches, because the handler was moved
// onto a ui.Keybindings field and the row was left behind. Rebinding the field
// then breaks the key while the help screen keeps advertising it.
//
// For every per-view help row it splits the key column into field-driven parts
// (which follow a rebind by construction) and literal parts, and requires each
// literal to appear as a real case label or == comparison in that view's
// dispatcher — or to carry a reasoned exemption.
//
// What it does NOT catch, by construction:
//   - the explorer (context-free) sections: their keys are dispatched across a
//     dozen files with mode routing this map cannot express;
//   - views dispatching on tea.KeyCode rather than msg.String() (see
//     helpLiteralHandlerFiles);
//   - a literal that is a case label in the right file but in an unreachable
//     branch, or under a different mode handled by the same file — the scan is
//     per-file, not per-function;
//   - the reverse drift, a handler key with no help row at all.
func TestHelpSections_LiteralKeysAreReallyLiteral(t *testing.T) {
	restoreWhichKeyGlobals(t)

	aliases := helpKeyDisplayAliases()
	exempt := helpLiteralExemptions()
	used := map[string]bool{}
	sentinel := sentinelKeybindings()

	contexts := make([]string, 0, len(helpLiteralHandlerFiles()))
	for ctx := range helpLiteralHandlerFiles() {
		contexts = append(contexts, ctx)
	}
	sort.Strings(contexts)

	for _, ctx := range contexts {
		handlerKeys := helpHandlerKeyLiterals(t, helpLiteralHandlerFiles()[ctx])

		ui.ActiveKeybindings = ui.DefaultKeybindings()
		real := ui.ViewerHelpRows(ctx)
		ui.ActiveKeybindings = sentinel
		sent := ui.ViewerHelpRows(ctx)

		if len(real) == 0 {
			t.Errorf("helpLiteralHandlerFiles names context %q, which has no help rows", ctx)
			continue
		}
		if len(real) != len(sent) {
			t.Fatalf("%s: help row count changed with the bindings (%d vs %d); the catalog must be static",
				ctx, len(real), len(sent))
		}

		for i := range real {
			realParts := strings.Split(real[i].Key, "/")
			sentParts := strings.Split(sent[i].Key, "/")
			if len(realParts) != len(sentParts) {
				continue // a binding's value contains "/"; parts no longer line up
			}
			for j, tok := range realParts {
				tok = strings.TrimSpace(tok)
				if tok == "" || strings.Contains(sentParts[j], "__hl_") {
					continue // empty, or field-driven and therefore rebind-safe
				}
				if key, ok := aliases[tok]; ok {
					tok = key
				}
				if handlerKeys[tok] {
					continue
				}
				switch {
				case exempt["*|"+tok] != "":
					used["*|"+tok] = true
				case exempt[ctx+"|"+tok] != "":
					used[ctx+"|"+tok] = true
				default:
					t.Errorf("%s help row %q hardcodes %q, which %v never matches as a literal; "+
						"point the row at the ui.Keybindings field the handler reads, or add a reasoned exemption for %q",
						ctx, real[i].Key, tok, helpLiteralHandlerFiles()[ctx], ctx+"|"+tok)
				}
			}
		}
	}

	for key := range exempt {
		if !used[key] {
			t.Errorf("helpLiteralExemptions names %q, which is not a mismatch anymore; remove the stale entry", key)
		}
	}
}
