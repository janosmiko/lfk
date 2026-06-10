package ui

import "testing"

// hide_badges resolves per context: a clusters.<ctx>.security.hide_badges entry
// overrides the global default, mirroring ResolveSecurityEnabled.
func TestResolveSecurityHideBadges(t *testing.T) {
	origGlobal := ConfigSecurityHideBadges
	origCluster := ConfigClusterSecurityHideBadges
	t.Cleanup(func() {
		ConfigSecurityHideBadges = origGlobal
		ConfigClusterSecurityHideBadges = origCluster
	})

	ConfigSecurityHideBadges = false
	ConfigClusterSecurityHideBadges = map[string]bool{
		"prod": true,
		"dev":  false,
	}

	if !ResolveSecurityHideBadges("prod") {
		t.Error("per-context true must override global false")
	}
	if ResolveSecurityHideBadges("dev") {
		t.Error("per-context false must be honored")
	}
	if ResolveSecurityHideBadges("staging") {
		t.Error("unlisted context must fall back to the global default (false)")
	}

	ConfigSecurityHideBadges = true
	if !ResolveSecurityHideBadges("staging") {
		t.Error("unlisted context must fall back to the global default (true)")
	}
}
