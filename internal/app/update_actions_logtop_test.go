package app

import (
	"testing"
)

func TestExecuteActionLogTop_SetsMode(t *testing.T) {
	m := basePush80Model()
	m.actionCtx.kind = "Deployment"
	m.actionCtx.name = "web"
	mdl, cmd := m.executeActionLogTop()
	got := mdl.(Model)
	if got.mode != modeLogTop {
		t.Errorf("mode = %v, want modeLogTop", got.mode)
	}
	if cmd == nil {
		t.Error("expected a log-stream command")
	}
	if got.logTop.title == "" {
		t.Error("expected a title to be set")
	}
}

// TestExecuteActionLogTop_NoContainerShowsAllContainers guards the bug where
// opening Log Top on a group resource (no container selected) set
// selectedContainers to [""], which filtered out every streamed line (no
// container is named "") and showed "0 matched / 0 unmatched". An empty
// selectedContainers means "show all".
func TestExecuteActionLogTop_NoContainerShowsAllContainers(t *testing.T) {
	m := basePush80Model()
	m.actionCtx.kind = "Deployment"
	m.actionCtx.name = "traefik"
	m.actionCtx.containerName = ""
	mdl, _ := m.executeActionLogTop()
	got := mdl.(Model)
	if len(got.logView.selectedContainers) != 0 {
		t.Fatalf("selectedContainers = %#v, want empty (show all) when no container selected", got.logView.selectedContainers)
	}
}

// TestExecuteActionLogTop_ContainerSelected pins a single container when one
// was chosen.
func TestExecuteActionLogTop_ContainerSelected(t *testing.T) {
	m := basePush80Model()
	m.actionCtx.kind = "Pod"
	m.actionCtx.name = "traefik-abc"
	m.actionCtx.containerName = "traefik"
	mdl, _ := m.executeActionLogTop()
	got := mdl.(Model)
	if len(got.logView.selectedContainers) != 1 || got.logView.selectedContainers[0] != "traefik" {
		t.Fatalf("selectedContainers = %#v, want [traefik]", got.logView.selectedContainers)
	}
}

// TestOpenLogTopFromViewer_SwitchesMode verifies that pressing the LogTop key
// in the log viewer switches mode to modeLogTop and populates logTop.rows.
func TestOpenLogTopFromViewer_SwitchesMode(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogs
	m.logView.title = "Logs: my-pod"
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/a","DownstreamStatus":200}`,
		`2026-06-18T10:00:01Z {"RequestMethod":"POST","RequestPath":"/b","DownstreamStatus":500}`,
	}
	mdl, cmd, handled := m.handleLogActionKey(key("T"))
	got := mdl.(Model)
	if !handled {
		t.Error("handleLogActionKey(T) returned handled=false")
	}
	if cmd != nil {
		t.Error("openLogTopFromViewer should return nil cmd (reuses existing stream)")
	}
	if got.mode != modeLogTop {
		t.Errorf("mode = %v, want modeLogTop", got.mode)
	}
	if len(got.logTop.rows) == 0 {
		t.Error("expected logTop.rows to be non-empty after opening from viewer")
	}
}

// TestOpenLogTopFromViewer_TitleStripped verifies the "Logs: " prefix is stripped.
func TestOpenLogTopFromViewer_TitleStripped(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogs
	m.logView.title = "Logs: traefik/web"
	m.logView.rawLines = []string{
		`2026-06-18T10:00:00Z {"RequestMethod":"GET","RequestPath":"/","DownstreamStatus":200}`,
	}
	mdl, _, _ := m.handleLogActionKey(key("T"))
	got := mdl.(Model)
	if got.logTop.title != "Log Top: traefik/web" {
		t.Errorf("logTop.title = %q, want %q", got.logTop.title, "Log Top: traefik/web")
	}
}
