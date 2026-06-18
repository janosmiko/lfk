package app

import "testing"

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
