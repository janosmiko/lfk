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
