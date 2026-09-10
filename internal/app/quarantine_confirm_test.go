package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

func TestQuarantineConfirmNotes_NamesEveryServiceAndTheReplacement(t *testing.T) {
	cases := []struct {
		name              string
		services          []string
		ownedByController bool
		wantRisk          string
	}{
		{"one service, owned", []string{"svc-a"}, true, "its controller creates a replacement pod"},
		{"two services, owned", []string{"svc-a", "svc-b"}, true, "its controller creates a replacement pod"},
		{"five services, orphan", []string{"svc-a", "svc-b", "svc-c", "svc-d", "svc-e"}, false, "nothing recreates this pod"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := quarantineState{services: tc.services, keys: []string{"app"}}
			notes := quarantineConfirmNotes(st, tc.ownedByController)

			var scope, risk string
			for _, n := range notes {
				switch n.Label {
				case "Scope":
					scope = n.Text
				case "Risk":
					risk = n.Text
					assert.True(t, n.Warn, "the risk row always warns, whichever branch it takes")
				}
			}

			for _, svc := range tc.services {
				assert.Contains(t, scope, svc)
			}
			assert.Equal(t, tc.wantRisk, risk)
		})
	}
}

func TestRenderOverlayConfirm_QuarantineShowsServices(t *testing.T) {
	m := Model{
		width:           80,
		height:          24,
		pendingAction:   model.ActionLabelQuarantine,
		confirmAction:   "web-1",
		confirmTitle:    "Confirm Quarantine",
		confirmQuestion: "Quarantine web-1?",
	}
	m.actionCtx.kind = "Pod"
	m.actionCtx.raw = map[string]any{
		"metadata": map[string]any{
			"ownerReferences": []any{
				map[string]any{"kind": "ReplicaSet", "name": "web", "controller": true},
			},
		},
	}
	m.quarantine.services = []string{"svc-web", "svc-web-internal"}
	m.quarantine.keys = []string{"app"}

	box := confirmBox(m)

	assert.Contains(t, box, "svc-web")
	assert.Contains(t, box, "svc-web-internal")
	assert.Contains(t, box, "its controller creates a replacement pod")
}
