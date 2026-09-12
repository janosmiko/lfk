package app

import (
	"reflect"
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
)

// The duplicate-key guards run under ui.DefaultKeybindings(), which is exactly
// the keybinding set that cannot produce the collision worth guarding: a
// binding rebound onto a key some viewer hardcodes. kb.Refresh = "r" (a k9s
// habit) meets the Object Explorer's literal "r" and one of the two rows
// becomes a lie. The sweeps below rebind on purpose.

// wkCatalogFields returns the ui.Keybindings field names a catalog actually
// reads, discovered by mutating one field at a time and watching for an entry
// whose key moves. Derived rather than listed so a new entry is swept the day
// it is added.
func wkCatalogFields(cat whichKeyCatalog) []string {
	base := ui.DefaultKeybindings()
	rt := reflect.TypeFor[ui.Keybindings]()
	entries := cat.entries()
	out := make([]string, 0, rt.NumField())
	for i := range rt.NumField() {
		if rt.Field(i).Type.Kind() != reflect.String {
			continue
		}
		mutated := base
		reflect.ValueOf(&mutated).Elem().Field(i).SetString("wksentinel")
		for _, e := range entries {
			if e.Key(base) != e.Key(mutated) {
				out = append(out, rt.Field(i).Name)
				break
			}
		}
	}
	return out
}

// wkCatalogLiterals returns the keys a catalog hardcodes: those that survive
// rewriting every single binding at once.
func wkCatalogLiterals(cat whichKeyCatalog) []string {
	base := ui.DefaultKeybindings()
	alt := base
	rv := reflect.ValueOf(&alt).Elem()
	for i := range rv.NumField() {
		if rv.Field(i).Kind() == reflect.String {
			rv.Field(i).SetString("wkalt" + rv.Type().Field(i).Name)
		}
	}
	seen := map[string]bool{}
	var out []string
	for _, e := range cat.entries() {
		k := e.Key(base)
		if k != "" && k == e.Key(alt) && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

// wkAssertNoDuplicateKeys is the assertion both sweeps share.
func wkAssertNoDuplicateKeys(t *testing.T, m Model, kb ui.Keybindings, what string) {
	t.Helper()
	byKey := map[string][]string{}
	for _, e := range m.availableWhichKeyActions() {
		k := e.Key(kb)
		byKey[k] = append(byKey[k], e.Label)
	}
	for k, labels := range byKey {
		if len(labels) > 1 {
			t.Errorf("%s: key %q is offered by %v at once; the handler runs exactly one of them, so the other row advertises a keystroke it does not own",
				what, k, labels)
		}
	}
}

// TestWhichKeyCatalogs_RebindOntoAHardcodedKeyIsNotAdvertisedTwice is the
// widened duplicate guard. Per catalog it rebinds every ui.Keybindings field
// the catalog reads onto every key the catalog hardcodes — the exact overlap
// the default set cannot produce — and requires the panel to advertise each key
// at most once.
func TestWhichKeyCatalogs_RebindOntoAHardcodedKeyIsNotAdvertisedTwice(t *testing.T) {
	restoreWhichKeyGlobals(t)

	swept := 0
	for _, mc := range whichKeyCatalogList {
		fields := wkCatalogFields(mc.catalog)
		literals := wkCatalogLiterals(mc.catalog)
		if len(fields) == 0 {
			t.Errorf("%s: no ui.Keybindings field discovered; the sweep would test nothing", mc.name)
			continue
		}
		scenarios := wkPairScenarios(t, mc.mode)
		t.Run(mc.name, func(t *testing.T) {
			for _, field := range fields {
				for _, lit := range literals {
					kb := ui.DefaultKeybindings()
					reflect.ValueOf(&kb).Elem().FieldByName(field).SetString(lit)
					ui.ActiveKeybindings = kb
					swept++
					for _, sc := range scenarios {
						wkAssertNoDuplicateKeys(t, sc.build(), kb, field+"="+lit+" / "+sc.name)
					}
				}
			}
		})
	}
	if swept == 0 {
		t.Fatal("no rebind was swept; the discovery helpers found no field/literal overlap to test")
	}
	t.Logf("swept %d field-onto-literal rebinds", swept)
}

// TestWhichKeyCatalogs_CollapsingEveryBindingLeavesNoDuplicate covers the
// explorer, which hardcodes no key at all (every entry reads a binding) and so
// has nothing for the sweep above to collide with. Pointing every field the
// catalog reads at one key is the degenerate rebind: whatever survives, no key
// may be advertised twice.
func TestWhichKeyCatalogs_CollapsingEveryBindingLeavesNoDuplicate(t *testing.T) {
	restoreWhichKeyGlobals(t)

	for _, mc := range whichKeyCatalogList {
		fields := wkCatalogFields(mc.catalog)
		kb := ui.DefaultKeybindings()
		rv := reflect.ValueOf(&kb).Elem()
		for _, f := range fields {
			rv.FieldByName(f).SetString("z")
		}
		ui.ActiveKeybindings = kb
		scenarios := wkPairScenarios(t, mc.mode)
		t.Run(mc.name, func(t *testing.T) {
			for _, sc := range scenarios {
				wkAssertNoDuplicateKeys(t, sc.build(), kb, "every binding on \"z\" / "+sc.name)
			}
		})
	}
}
