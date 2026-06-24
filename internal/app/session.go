package app

import (
	"os"
	"path/filepath"
	"slices"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/paths"
)

// SessionTab represents the persisted navigation state for a single tab.
type SessionTab struct {
	Context            string   `json:"context" yaml:"context"`
	Namespace          string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	AllNamespaces      bool     `json:"all_namespaces,omitempty" yaml:"all_namespaces,omitempty"`
	SelectedNamespaces []string `json:"selected_namespaces,omitempty" yaml:"selected_namespaces,omitempty"`
	NsSelectionNegated bool     `json:"ns_selection_negated,omitempty" yaml:"ns_selection_negated,omitempty"`
	ResourceType       string   `json:"resource_type,omitempty" yaml:"resource_type,omitempty"`
	ResourceName       string   `json:"resource_name,omitempty" yaml:"resource_name,omitempty"`
	// List view state at quit time, restored once the resource list reloads.
	Filter          string `json:"filter,omitempty" yaml:"filter,omitempty"`
	FilterBroad     bool   `json:"filter_broad,omitempty" yaml:"filter_broad,omitempty"`
	CursorName      string `json:"cursor_name,omitempty" yaml:"cursor_name,omitempty"`
	CursorNamespace string `json:"cursor_namespace,omitempty" yaml:"cursor_namespace,omitempty"`
}

// SessionState represents the persisted navigation state across restarts.
type SessionState struct {
	// Legacy single-tab fields (kept for backward compatibility on load).
	Context            string   `json:"context" yaml:"context"`
	Namespace          string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	AllNamespaces      bool     `json:"all_namespaces,omitempty" yaml:"all_namespaces,omitempty"`
	SelectedNamespaces []string `json:"selected_namespaces,omitempty" yaml:"selected_namespaces,omitempty"`
	NsSelectionNegated bool     `json:"ns_selection_negated,omitempty" yaml:"ns_selection_negated,omitempty"`
	ResourceType       string   `json:"resource_type,omitempty" yaml:"resource_type,omitempty"` // group/version/resource ref string
	ResourceName       string   `json:"resource_name,omitempty" yaml:"resource_name,omitempty"`

	// List view state for the legacy single-tab shape; the multi-tab path
	// carries these per SessionTab instead.
	Filter          string `json:"filter,omitempty" yaml:"filter,omitempty"`
	FilterBroad     bool   `json:"filter_broad,omitempty" yaml:"filter_broad,omitempty"`
	CursorName      string `json:"cursor_name,omitempty" yaml:"cursor_name,omitempty"`
	CursorNamespace string `json:"cursor_namespace,omitempty" yaml:"cursor_namespace,omitempty"`

	// Multi-tab fields.
	Tabs      []SessionTab `json:"tabs,omitempty" yaml:"tabs,omitempty"`
	ActiveTab int          `json:"active_tab,omitempty" yaml:"active_tab,omitempty"`
}

// sessionFilePath returns the path to the session state file.
// Resolves the lfk state directory via internal/paths.
func sessionFilePath() string {
	dir, err := paths.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "session.yaml")
}

// migrateStateFile checks if a state file exists at the legacy ~/.config/lfk/ location
// and migrates it to the new XDG state directory. Returns the file data if found, nil otherwise.
func migrateStateFile(filename, newPath string) []byte {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	legacyPath := filepath.Join(home, ".config", "lfk", filename)
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return nil
	}
	// Migrate: write to new location and remove legacy file.
	if err := os.MkdirAll(filepath.Dir(newPath), 0o700); err == nil {
		if os.WriteFile(newPath, data, 0o600) == nil {
			_ = os.Remove(legacyPath)
		}
	}
	return data
}

// loadSession reads session state from disk. Returns nil on any error.
// Falls back to the legacy ~/.config/lfk/ location and migrates if found.
func loadSession() *SessionState {
	path := sessionFilePath()
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// Try legacy location and migrate.
		data = migrateStateFile("session.yaml", path)
		if data == nil {
			return nil
		}
	}
	var s SessionState
	if err := yaml.Unmarshal(data, &s); err != nil {
		logger.Warn("Session file is corrupt; starting fresh", "error", err, "path", path)
		return nil
	}
	// A session without a context is not useful.
	if s.Context == "" {
		return nil
	}
	return &s
}

// saveSession writes session state to disk.
func saveSession(s SessionState) error {
	path := sessionFilePath()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// saveCurrentSession persists the current navigation state to the session file.
func (m *Model) saveCurrentSession() {
	if m.unionMode {
		return
	}
	// Ensure the active tab's state is up to date before serialising.
	m.saveCurrentTab()

	s := SessionState{
		ActiveTab: m.activeTab,
	}

	for _, t := range m.tabs {
		st := SessionTab{
			Context:       t.nav.Context,
			AllNamespaces: t.allNamespaces,
		}
		if !t.allNamespaces {
			st.Namespace = t.namespace
			if len(t.selectedNamespaces) > 0 {
				nsList := make([]string, 0, len(t.selectedNamespaces))
				for ns := range t.selectedNamespaces {
					nsList = append(nsList, ns)
				}
				slices.Sort(nsList)
				st.SelectedNamespaces = nsList
			}
			st.NsSelectionNegated = t.nsSelectionNegated
		}
		if t.nav.ResourceType.Resource != "" {
			st.ResourceType = t.nav.ResourceType.ResourceRef()
		}
		if t.nav.ResourceName != "" {
			st.ResourceName = t.nav.ResourceName
		}
		// Persist the list filter and highlighted row only when the tab sits on
		// a resource list; a filter typed at the resource-types level is not
		// meaningful once the session reopens directly into the resource view.
		// saveCurrentTab captured the cursor identity for every tab (live for
		// the active one, last-seen for the rest), so restore can reopen each
		// tab on its own row.
		if t.nav.Level == model.LevelResources {
			st.Filter = t.filterText
			st.FilterBroad = t.filterBroadMode
			st.CursorName = t.cursorName
			st.CursorNamespace = t.cursorNamespace
		}
		s.Tabs = append(s.Tabs, st)
	}

	// Legacy compat: set top-level context to active tab's context.
	if len(s.Tabs) > 0 && s.ActiveTab < len(s.Tabs) {
		s.Context = s.Tabs[s.ActiveTab].Context
	}

	// Session persistence is best-effort, but a write failure means the
	// next start won't restore the active context — log it so users can
	// diagnose disk-full / permissions issues from lfk.log.
	if err := saveSession(s); err != nil {
		logger.Error("Failed to persist session state", "error", err)
	}
}
