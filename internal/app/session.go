package app

import (
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"
)

// SessionState represents the persisted navigation state across restarts.
type SessionState struct {
	Context            string   `json:"context" yaml:"context"`
	Namespace          string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	AllNamespaces      bool     `json:"all_namespaces,omitempty" yaml:"all_namespaces,omitempty"`
	SelectedNamespaces []string `json:"selected_namespaces,omitempty" yaml:"selected_namespaces,omitempty"`
	ResourceType       string   `json:"resource_type,omitempty" yaml:"resource_type,omitempty"` // group/version/resource ref string
	ResourceName       string   `json:"resource_name,omitempty" yaml:"resource_name,omitempty"`
}

// sessionFilePath returns the path to the session state file.
// Uses $XDG_STATE_HOME/lfk/ (defaults to ~/.local/state/lfk/) per XDG specification.
func sessionFilePath() string {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "lfk", "session.yaml")
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
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err == nil {
		if os.WriteFile(newPath, data, 0o644) == nil {
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// saveCurrentSession persists the current navigation state to the session file.
func (m *Model) saveCurrentSession() {
	s := SessionState{
		Context:       m.nav.Context,
		AllNamespaces: m.allNamespaces,
	}
	if !m.allNamespaces {
		s.Namespace = m.namespace
		if len(m.selectedNamespaces) > 0 {
			for ns := range m.selectedNamespaces {
				s.SelectedNamespaces = append(s.SelectedNamespaces, ns)
			}
		}
	}
	if m.nav.ResourceType.Resource != "" {
		s.ResourceType = m.nav.ResourceType.ResourceRef()
	}
	if m.nav.ResourceName != "" {
		s.ResourceName = m.nav.ResourceName
	}
	// Fire and forget; session persistence is best-effort.
	_ = saveSession(s)
}
