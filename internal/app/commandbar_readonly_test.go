package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/ui"
)

func TestCommandBarReadOnly_Kubectl(t *testing.T) {
	cases := []struct {
		input   string
		blocked bool
	}{
		{"k delete pod x", true},
		{"k apply -f x.yaml", true},
		{"k edit deploy x", true},
		{"k scale deploy x --replicas=0", true},
		{"kubectl delete pod x", true},
		{"k -n ns delete pod x", true},
		{"k --namespace=ns delete pod x", true},
		{"k --context test-ctx delete pod x", true},
		{"k exec -it x -- sh", true},
		{"k config use-context other", true},
		{"k auth reconcile -f x", true},
		{"k rollout restart deploy x", true},
		{"k rollout undo deploy x", true},
		{"k -n ns rollout pause deploy x", true},
		{"k somepluginverb", true},
		{"k -n ns", true},
		{"k get pods", false},
		{"k -n ns get pods", false},
		{"k --namespace=ns describe pod x", false},
		{"k logs x", false},
		{"k top pods", false},
		{"k explain pod", false},
		{"k api-resources", false},
		{"k api-versions", false},
		{"k version", false},
		{"k cluster-info", false},
		{"k events", false},
		{"k diff -f x.yaml", false},
		{"k auth can-i get pods", false},
		{"k config view", false},
		{"k config get-contexts", false},
		{"k config current-context", false},
		{"k rollout status deploy x", false},
		{"k -n ns rollout history deploy x", false},
	}
	for _, ro := range []string{"cli", "ctx", "none"} {
		for _, tc := range cases {
			t.Run(ro+"/"+tc.input, func(t *testing.T) {
				m := baseModelWithFakeClient()
				switch ro {
				case "cli":
					m.cliReadOnly = true
					m.readOnly = true
				case "ctx":
					m.readOnly = true
				}
				result, cmd := m.executeCommandBarInput(tc.input)
				rm := result.(Model)
				if ro != "none" && tc.blocked {
					assert.True(t, rm.statusMessageErr)
					assert.Contains(t, rm.statusMessage, "Read-only mode")
					return
				}
				assert.NotNil(t, cmd)
				assert.Empty(t, rm.statusMessage)
			})
		}
	}
}

func TestCommandBarReadOnly_ExplicitContext(t *testing.T) {
	prev := ui.ConfigClusterReadOnly
	t.Cleanup(func() { ui.ConfigClusterReadOnly = prev })
	ui.ConfigClusterReadOnly = map[string]bool{"prod": true}

	t.Run("writable_current_read_only_target", func(t *testing.T) {
		for _, in := range []string{"k --context prod delete pod x", "k --context=prod delete pod x"} {
			m := baseModelWithFakeClient()
			r, _ := m.executeCommandBarInput(in)
			assert.Contains(t, r.(Model).statusMessage, "Read-only mode", in)
		}
	})
	t.Run("read_only_current_writable_target", func(t *testing.T) {
		m := baseModelWithFakeClient()
		m.readOnly = true
		r, _ := m.executeCommandBarInput("k --context dev delete pod x")
		assert.Contains(t, r.(Model).statusMessage, "Read-only mode")
	})
	t.Run("read_only_target_read_verb_runs", func(t *testing.T) {
		m := baseModelWithFakeClient()
		r, cmd := m.executeCommandBarInput("k --context prod get pods")
		assert.NotNil(t, cmd)
		assert.Empty(t, r.(Model).statusMessage)
	})
	t.Run("session_override_unlocks_target", func(t *testing.T) {
		m := baseModelWithFakeClient()
		m.contextROOverrides = map[string]bool{"prod": false}
		r, cmd := m.executeCommandBarInput("k --context prod delete pod x")
		assert.NotNil(t, cmd)
		assert.Empty(t, r.(Model).statusMessage)
	})
}

func TestCommandBarReadOnly_Shell(t *testing.T) {
	for _, tc := range []struct {
		name    string
		cli, ro bool
		blocked bool
	}{
		{"cli_flag", true, true, true},
		{"context", false, true, true},
		{"writable", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := baseModelWithFakeClient()
			m.cliReadOnly, m.readOnly = tc.cli, tc.ro
			result, cmd := m.executeCommandBarInput("! kubectl delete pod x")
			rm := result.(Model)
			if tc.blocked {
				assert.True(t, rm.statusMessageErr)
				assert.Equal(t, readOnlyBlockedMessage("shell commands"), rm.statusMessage)
				return
			}
			assert.NotNil(t, cmd)
			assert.Empty(t, rm.statusMessage)
		})
	}
}
