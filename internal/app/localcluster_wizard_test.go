package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// openManagerForTest opens the manager overlay (without firing Detect)
// so wizard tests can drive only the wizard transitions.
func openManagerForTest(t *testing.T) Model {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := NewModel(k8s.NewTestClient(nil, nil), StartupOptions{})
	m.nav.Level = model.LevelClusters
	m.overlay = overlayLocalClusters
	m.localClusterState.screen = localClusterScreenList
	return m
}

func TestWizard_NKeyEntersProviderPicker(t *testing.T) {
	m := openManagerForTest(t)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	rm := updated.(Model)
	if rm.localClusterState.screen != localClusterScreenWizardProvider {
		t.Fatalf("screen = %v, want WizardProvider", rm.localClusterState.screen)
	}
	// installedSet may be empty in tests (no kind/k3d/minikube in PATH)
	// -- that's fine for compile-time check.
	if rm.localClusterState.wizard.providerCur != 0 {
		t.Fatal("providerCur should default to 0")
	}
}

func TestWizard_ProviderPickerJK_MovesCursor(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardProvider
	m.localClusterState.wizard.installedSet = []string{"kind", "k3d"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if updated.(Model).localClusterState.wizard.providerCur != 1 {
		t.Fatalf("expected providerCur=1 after j")
	}
}

func TestWizard_ProviderPickerEnter_AdvancesToName(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardProvider
	m.localClusterState.wizard.installedSet = []string{"kind", "k3d"}
	m.localClusterState.wizard.providerCur = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rm := updated.(Model)
	if rm.localClusterState.screen != localClusterScreenWizardName {
		t.Fatalf("screen = %v, want WizardName", rm.localClusterState.screen)
	}
	if rm.localClusterState.wizard.provider != "kind" {
		t.Fatalf("provider = %q, want kind", rm.localClusterState.wizard.provider)
	}
}

func TestWizard_ProviderPickerEsc_BacksToList(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardProvider
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	rm := updated.(Model)
	if rm.localClusterState.screen != localClusterScreenList {
		t.Fatalf("screen = %v, want List", rm.localClusterState.screen)
	}
	if rm.overlay != overlayLocalClusters {
		t.Fatal("overlay must remain open")
	}
}

func TestWizard_NameTyping_BuildsBuffer(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardName
	m.localClusterState.wizard.provider = "kind"

	for _, r := range "dev" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	if m.localClusterState.wizard.name != "dev" {
		t.Fatalf("name = %q, want dev", m.localClusterState.wizard.name)
	}
}

func TestWizard_NameValidation_RejectsInvalid(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardName
	m.localClusterState.wizard.provider = "kind"

	for _, r := range "BAD" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	if m.localClusterState.wizard.name != "" {
		t.Fatalf("invalid runes should be filtered, got %q", m.localClusterState.wizard.name)
	}
}

func TestWizard_NameValidation_FlagsConflict(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.clusters = []localClusterRow{{Provider: "kind", Name: "dev"}}
	m.localClusterState.screen = localClusterScreenWizardName
	m.localClusterState.wizard.provider = "kind"

	for _, r := range "dev" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(Model)
	}
	if m.localClusterState.wizard.nameErr == "" {
		t.Fatal("expected nameErr for conflicting name")
	}
}

func TestWizard_NameEnter_BlocksWhenInvalid(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardName
	m.localClusterState.wizard.provider = "kind"
	m.localClusterState.wizard.name = ""

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rm := updated.(Model)
	if rm.localClusterState.screen != localClusterScreenWizardName {
		t.Fatal("Enter must not advance when name is empty")
	}
}

func TestWizard_NameEnter_AdvancesWhenValid(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardName
	m.localClusterState.wizard.provider = "kind"
	m.localClusterState.wizard.name = "dev"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rm := updated.(Model)
	if rm.localClusterState.screen != localClusterScreenWizardVersion {
		t.Fatalf("screen = %v, want WizardVersion", rm.localClusterState.screen)
	}
}

func TestWizard_NameEsc_BacksToProvider(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardName
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	rm := updated.(Model)
	if rm.localClusterState.screen != localClusterScreenWizardProvider {
		t.Fatalf("screen = %v, want WizardProvider", rm.localClusterState.screen)
	}
}

func TestWizard_NameBackspace(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardName
	m.localClusterState.wizard.name = "dev"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	rm := updated.(Model)
	if rm.localClusterState.wizard.name != "de" {
		t.Fatalf("name = %q, want de", rm.localClusterState.wizard.name)
	}
}

func TestWizard_Version_EmptyAllowed(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardVersion
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rm := updated.(Model)
	if rm.localClusterState.screen != localClusterScreenWizardNodes {
		t.Fatalf("Enter on empty version must advance, got screen=%v", rm.localClusterState.screen)
	}
}

func TestWizard_Version_RejectsBadFormat(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardVersion
	m.localClusterState.wizard.version = "garbage"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rm := updated.(Model)
	if rm.localClusterState.screen != localClusterScreenWizardVersion {
		t.Fatal("Enter on bad version must block")
	}
	if rm.localClusterState.wizard.versionErr == "" {
		t.Fatal("versionErr must be populated")
	}
}

func TestWizard_Version_AcceptsValid(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardVersion
	m.localClusterState.wizard.version = "v1.30.0"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rm := updated.(Model)
	if rm.localClusterState.screen != localClusterScreenWizardNodes {
		t.Fatalf("Enter on valid version must advance, got %v", rm.localClusterState.screen)
	}
}

func TestWizard_Nodes_DefaultsToOne(t *testing.T) {
	m := openManagerForTest(t)
	m = m.startWizard()
	if m.localClusterState.wizard.nodesStr != "1" {
		t.Fatalf("nodesStr = %q, want 1", m.localClusterState.wizard.nodesStr)
	}
}

func TestWizard_Nodes_RejectsZero(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardNodes
	m.localClusterState.wizard.nodesStr = "0"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rm := updated.(Model)
	if rm.localClusterState.screen != localClusterScreenWizardNodes {
		t.Fatal("Enter on nodes=0 must block")
	}
}

func TestWizard_Nodes_AcceptsAdvances(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardNodes
	m.localClusterState.wizard.nodesStr = "3"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rm := updated.(Model)
	if rm.localClusterState.screen != localClusterScreenWizardConfirm {
		t.Fatalf("Enter on valid nodes must advance, got %v", rm.localClusterState.screen)
	}
	if rm.localClusterState.wizard.nodes != 3 {
		t.Fatalf("nodes = %d, want 3", rm.localClusterState.wizard.nodes)
	}
}

func TestWizard_VersionEsc_BacksToName(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardVersion
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	rm := updated.(Model)
	if rm.localClusterState.screen != localClusterScreenWizardName {
		t.Fatalf("screen = %v, want WizardName", rm.localClusterState.screen)
	}
}

func TestWizard_ConfirmEsc_BacksToNodes(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardConfirm
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	rm := updated.(Model)
	if rm.localClusterState.screen != localClusterScreenWizardNodes {
		t.Fatalf("screen = %v, want WizardNodes", rm.localClusterState.screen)
	}
}

func TestWizard_ConfirmEnter_DispatchesCreateCmd(t *testing.T) {
	m := openManagerForTest(t)
	m.localClusterState.screen = localClusterScreenWizardConfirm
	m.localClusterState.wizard = localClusterWizard{
		provider: "kind", name: "dev", version: "v1.30.0", nodes: 1, nodesStr: "1",
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rm := updated.(Model)
	if cmd == nil {
		t.Fatal("Enter on confirm must return a tea.Cmd")
	}
	if rm.localClusterState.screen != localClusterScreenList {
		t.Fatal("Confirm dispatch must return to list view")
	}
}
