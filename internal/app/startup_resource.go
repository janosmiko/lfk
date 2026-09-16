package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// effectiveStartupResource resolves the requested startup resource: the
// --resource flag wins over the startup_resource config key.
func effectiveStartupResource(opts StartupOptions) string {
	if opts.Resource != "" {
		return opts.Resource
	}
	return ui.ConfigStartupResource
}

// applyStartupResource resets the active tab's list state and points it at
// the requested resource. A nil sess mirrors the HasCLIOverrides branch in
// NewModel: no saved workspace, so a synthetic one-tab session is built.
func applyStartupResource(sess *SessionState, resource, contextName, defaultNS string, allNamespaces bool) *SessionState {
	if sess == nil {
		tab := SessionTab{Context: contextName, AllNamespaces: allNamespaces, ResourceType: resource}
		if !allNamespaces {
			tab.Namespace = defaultNS
		}
		return &SessionState{Context: contextName, Tabs: []SessionTab{tab}}
	}
	if len(sess.Tabs) > 0 {
		idx := sess.ActiveTab
		if idx < 0 || idx >= len(sess.Tabs) {
			idx = 0
		}
		sess.Tabs[idx].ResourceType = resource
		sess.Tabs[idx].ResourceName = ""
		sess.Tabs[idx].Filter = ""
		sess.Tabs[idx].FilterBroad = false
		sess.Tabs[idx].CursorName = ""
		sess.Tabs[idx].CursorNamespace = ""
		return sess
	}
	sess.ResourceType = resource
	sess.ResourceName = ""
	sess.Filter = ""
	sess.FilterBroad = false
	sess.CursorName = ""
	sess.CursorNamespace = ""
	return sess
}

// resourceNameMatches is shared by the `:resource` command bar jump and
// startup resource resolution so the two stay in sync.
func resourceNameMatches(resolved, targetGroup, itemResource, itemName, itemKind, itemGroup string) bool {
	nameMatch := itemResource == resolved || toSingular(itemResource) == resolved ||
		itemName == resolved || toSingular(itemName) == resolved ||
		itemKind == resolved
	if !nameMatch {
		return false
	}
	if targetGroup == "" {
		return true
	}
	return strings.Contains(itemGroup, targetGroup)
}

// groupFromExtra extracts the API group (lowercased) from an Extra field
// shaped "group/version/resource" or "v1/resource" (core group).
func groupFromExtra(extra string) string {
	parts := strings.Split(extra, "/")
	if len(parts) >= 3 {
		return strings.ToLower(parts[0])
	}
	if len(parts) == 2 {
		return "core"
	}
	return ""
}

// resolveResourceTypeByName resolves a typed resource name (short plural,
// singular, Kind, or "name.group" for CRDs), the fallback for a session
// ResourceType that isn't a "group/version/resource" ref.
func resolveResourceTypeByName(name string, discovered []model.ResourceTypeEntry) (model.ResourceTypeEntry, bool) {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return model.ResourceTypeEntry{}, false
	}
	resolved := lower
	if ui.SearchAbbreviations != nil {
		if full, ok := ui.SearchAbbreviations[lower]; ok {
			resolved = strings.ToLower(full)
		}
	}
	var targetGroup string
	if dotIdx := strings.Index(lower, "."); dotIdx > 0 {
		targetGroup = strings.ToLower(lower[dotIdx+1:])
		resolved = strings.ToLower(lower[:dotIdx])
	}
	for _, rt := range discovered {
		itemGroup := strings.ToLower(rt.APIGroup)
		if itemGroup == "" {
			itemGroup = "core"
		}
		itemResource := strings.ToLower(rt.Resource)
		itemKind := strings.ToLower(rt.Kind)
		if resourceNameMatches(resolved, targetGroup, itemResource, itemResource, itemKind, itemGroup) {
			return rt, true
		}
	}
	return model.ResourceTypeEntry{}, false
}
