package app

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
)

// helpSectionGroups maps every context-free help section (help_sections.go) to
// the which-key group (whichkey_registry.go) that is the same concept under a
// different name. The empty group means "this section has no which-key
// counterpart by design": the panel is a discovery aid for actions available
// right now, while the help screen is the full reference, so navigation, the
// command bar's own sub-keys, and the overlay-local blocks legitimately exist
// only on the help side.
func helpSectionGroups() map[string]whichKeyGroup {
	return map[string]whichKeyGroup{
		"Search & Filter":  wkFilter,
		"Views & Tools":    wkViews,
		"Sorting":          wkSort,
		"Multi-Selection":  wkSelection,
		"Actions":          wkActions,
		"Modes & Settings": wkSettings,

		"Navigation":  "",
		"Command Bar": "",
		"Bookmarks":   "",
		"Tabs":        "",
		"Mouse":       "",
		"Help View":   "",
		"General":     "",
	}
}

// helpGroupExemptions lists the (binding, help section) pairs that carry a
// different group on each surface on purpose, keyed "Field@Section". Every
// entry needs a reason written from the handler or the section's own scope.
//
// All four are the same shape: a help section that documents one mode,
// overlay, or input device end to end, whose first (or last) row is the key
// that gets you into it. The which-key panel has no such structure — it is
// one flat list of what the current row can do — so it files those keys by
// what they DO, and the two answers differ without either being wrong.
func helpGroupExemptions() map[string]string {
	return map[string]string{
		// The canonical Action-menu row already sits in Actions and agrees
		// with the registry. This second row documents the same key's
		// selection-mode behaviour: openActionMenu (update_actions.go)
		// branches to openBulkSelectionMenu when hasSelection() is true, so
		// the Multi-Selection block would be incomplete without it.
		"ActionMenu@Multi-Selection": "second row for the bulk-menu branch openActionMenu takes under a selection; the canonical row is in Actions",

		// The section IS the command bar's keymap — every row below this one
		// (":pod", "tab", "ctrl+n/ctrl+p", ...) only applies once the bar is
		// open. Hoisting the opener into Search & Filter would leave a
		// section documenting a mode with no way into it.
		"CommandBar@Command Bar": "opens the mode the rest of the section documents; the registry files it under Filter because the bar's most-used verb is jumping/filtering by type",

		// Same shape: the Bookmarks section's remaining rows (j/k, "/",
		// enter, ctrl+x, alt+x) are all overlay-local and only reachable
		// after this key. The registry files it under Views because
		// handleKeyOpenMarks (update_keys.go) opens an overlay view.
		"OpenMarks@Bookmarks": "opens the overlay the rest of the section documents; the registry files it under Views because it opens an overlay view",

		// The Mouse section is device-scoped: click/wheel/drag rows that only
		// work while capture is on. toggleMouseCapture (update_mouse.go) is
		// what turns that on and off, so it anchors the block. The registry
		// files it under Settings alongside the other sticky mode toggles.
		"MouseToggle@Mouse": "enables/suspends the capture every other row in the section depends on; the registry files it under Settings with the other mode toggles",
	}
}

// helpAlignmentRow pairs one help row with the section title as the user sees
// it. Titles can embed a key ("Command Bar (:)"), so the title is read from a
// pass over the real bindings while the key column is read from a pass over
// sentinel bindings.
type helpAlignmentRow struct {
	section string
	key     string
}

// helpRowsWithFields resolves every explorer help row to the ui.Keybindings
// fields its key column reads. Rows whose key is a literal ("space", "esc")
// resolve to no field and are skipped: nothing ties them back to a registry
// entry.
func helpRowsWithFields(t *testing.T) map[string][]helpAlignmentRow {
	t.Helper()

	kb := ui.Keybindings{}
	v := reflect.ValueOf(&kb).Elem()
	fieldForSentinel := map[string]string{}
	for f, fv := range v.Fields() {
		if f.Type.Kind() != reflect.String {
			continue
		}
		sentinel := "__wk_" + f.Name + "__"
		fv.SetString(sentinel)
		fieldForSentinel[sentinel] = f.Name
	}

	original := ui.ActiveKeybindings
	t.Cleanup(func() { ui.ActiveKeybindings = original })

	ui.ActiveKeybindings = ui.DefaultKeybindings()
	titled := ui.ExplorerHelpRows()
	ui.ActiveKeybindings = kb
	sentinelled := ui.ExplorerHelpRows()

	if len(titled) != len(sentinelled) {
		t.Fatalf("help row count changed with the bindings (%d vs %d); the catalog must be static", len(titled), len(sentinelled))
	}

	out := map[string][]helpAlignmentRow{}
	for i, row := range sentinelled {
		for sentinel, field := range fieldForSentinel {
			if strings.Contains(row.Key, sentinel) {
				out[field] = append(out[field], helpAlignmentRow{section: titled[i].Section, key: titled[i].Key})
			}
		}
	}
	return out
}

// sectionGroup looks a section title up in helpSectionGroups, tolerating the
// one title that embeds its own key ("Command Bar (:)").
func sectionGroup(t *testing.T, title string) (whichKeyGroup, bool) {
	t.Helper()
	groups := helpSectionGroups()
	if g, ok := groups[title]; ok {
		return g, true
	}
	if base, _, found := strings.Cut(title, " ("); found {
		if g, ok := groups[base]; ok {
			return g, true
		}
	}
	return "", false
}

// TestHelpSections_GroupsMatchWhichKeyRegistry is the forcing function against
// taxonomy drift between the two keybinding surfaces. A binding listed in both
// the which-key registry and the help screen must sit in sections that mean
// the same thing (helpSectionGroups translates the two vocabularies), or carry
// a reasoned exemption. Without it the two catalogs were free to disagree
// silently — which they did: namespace selection was Filter in the panel and
// Actions in the help.
func TestHelpSections_GroupsMatchWhichKeyRegistry(t *testing.T) {
	restoreWhichKeyGlobals(t)

	registryGroup := whichKeyRegistryGroupsByField(t)
	helpRows := helpRowsWithFields(t)
	exempt := helpGroupExemptions()
	used := map[string]bool{}

	fields := make([]string, 0, len(helpRows))
	for f := range helpRows {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	for _, field := range fields {
		group, registered := registryGroup[field]
		if !registered {
			continue // help-only binding (navigation, tab management, ...)
		}
		for _, row := range helpRows[field] {
			want, known := sectionGroup(t, row.section)
			if !known {
				t.Errorf("help section %q has no entry in helpSectionGroups; map it to a which-key group or to \"\"", row.section)
				continue
			}
			if want == group {
				continue
			}
			key := field + "@" + strings.SplitN(row.section, " (", 2)[0]
			if reason, ok := exempt[key]; ok {
				used[key] = true
				if strings.TrimSpace(reason) == "" {
					t.Errorf("exemption %q has no reason", key)
				}
				continue
			}
			t.Errorf("binding %s is %q in the which-key registry but sits in help section %q (%q); move it or add a reasoned exemption for %q",
				field, group, row.section, want, key)
		}
	}

	for key := range exempt {
		if !used[key] {
			t.Errorf("helpGroupExemptions names %q, which is not a disagreement anymore; remove the stale entry", key)
		}
	}

	// Every explorer section must be mapped, so a new section cannot appear
	// without someone deciding whether it has a which-key counterpart.
	seen := map[string]bool{}
	for _, row := range ui.ExplorerHelpRows() {
		if seen[row.Section] {
			continue
		}
		seen[row.Section] = true
		if _, known := sectionGroup(t, row.Section); !known {
			t.Errorf("help section %q is not listed in helpSectionGroups", row.Section)
		}
	}
	for title := range helpSectionGroups() {
		found := false
		for section := range seen {
			if section == title || strings.HasPrefix(section, title+" (") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("helpSectionGroups names section %q, which the help catalog no longer has", title)
		}
	}
}

// whichKeyRegistryGroupsByField resolves each registered catalog entry back to
// the ui.Keybindings field it reads, so the two surfaces can be compared by
// field name rather than by the key value of the moment.
func whichKeyRegistryGroupsByField(t *testing.T) map[string]whichKeyGroup {
	t.Helper()

	kb := ui.Keybindings{}
	v := reflect.ValueOf(&kb).Elem()
	fieldForSentinel := map[string]string{}
	for f, fv := range v.Fields() {
		if f.Type.Kind() != reflect.String {
			continue
		}
		sentinel := "__wk_" + f.Name + "__"
		fv.SetString(sentinel)
		fieldForSentinel[sentinel] = f.Name
	}

	// Swept across every catalog, not just the explorer's: a binding shared by
	// two modes (kb.Search, kb.Refresh, kb.TogglePreview, ...) must sit in the
	// same group in both, or the panel's legend means one thing in the
	// explorer and another in a viewer.
	out := map[string]whichKeyGroup{}
	for _, mc := range whichKeyCatalogList {
		for _, e := range mc.catalog.entries() {
			field, ok := fieldForSentinel[e.Key(kb)]
			if !ok {
				continue // literal key (wkLiteralKey) — no source field
			}
			if prev, dup := out[field]; dup && prev != e.Group {
				t.Errorf("binding %s is registered under two groups (%q and %q); %s disagrees", field, prev, e.Group, mc.name)
			}
			out[field] = e.Group
		}
	}
	return out
}
