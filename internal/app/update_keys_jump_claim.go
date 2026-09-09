package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// claimJumpState remembers which pod and claim index handleExplorerActionKeyJumpClaim
// last jumped to, so a repeated press on the same pod advances through its
// remaining claims instead of always landing on the first one.
type claimJumpState struct {
	podKey string // "namespace/name" of the pod the last jump started from
	index  int    // claim index to use on the next press for that pod
}

// Repeated presses on the same pod cycle through its "claim:N" columns
// (populatePodResourceClaims), wrapping back to the first after the last.
func (m Model) handleExplorerActionKeyJumpClaim() (tea.Model, tea.Cmd, bool) {
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil, true
	}

	var claims []string
	hasUnresolvedClaims := false
	for _, kv := range sel.Columns {
		if strings.HasPrefix(kv.Key, "claim:") {
			claims = append(claims, kv.Value)
		}
		if kv.Key == "Resource Claims" {
			hasUnresolvedClaims = true
		}
	}
	if len(claims) == 0 {
		if hasUnresolvedClaims {
			m.setStatusMessage("Resource claim not created yet", true)
		} else {
			m.setStatusMessage("No resource claims on this pod", true)
		}
		return m, scheduleStatusClear(), true
	}

	podKey := sel.Namespace + "/" + sel.Name
	idx := 0
	if m.claimJump.podKey == podKey && m.claimJump.index < len(claims) {
		idx = m.claimJump.index
	}
	next := idx + 1
	if next >= len(claims) {
		next = 0
	}
	m.claimJump = claimJumpState{podKey: podKey, index: next}

	ret, cmd := m.navigateToOwner("ResourceClaim", claims[idx], "resource.k8s.io/v1")
	return ret, cmd, true
}
