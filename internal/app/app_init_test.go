package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
)

// TestNewModel_StartsInLoadingState guards against the startup empty-state
// flash. Init() dispatches loadContexts() asynchronously, so Bubbletea renders
// the first frame before any contextsLoadedMsg arrives. If the model is
// constructed with loading=false, that frame falls through to the empty-state
// messages ("No items", "No resource types found") until the reply lands.
// Constructing with loading=true makes the first render show the spinner; the
// loaded-message handlers (updateContextsLoaded, updateResourceTypes) clear the
// flag once real data arrives.
func TestNewModel_StartsInLoadingState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	client := k8s.NewTestClient(nil, nil)
	m := NewModel(client, StartupOptions{})
	if !m.loading {
		t.Fatal("NewModel must start in the loading state so the first render " +
			"shows the spinner instead of flashing the empty-state messages")
	}
}
