package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestInformerCacheConstants_MatchK8sPackage guards against silent drift
// between ui.InformerCache* and k8s.InformerCache*. The duplication is
// intentional (the comment in config.go explains: keep ui free of a k8s
// dependency in the production graph), but if anyone changes one set of
// strings without the other, main.go's k8s.InformerCacheMode(ui.Config…)
// cast would silently land on the auto-fallback in SetInformerCacheMode
// instead of the mode the user picked. This test fails loudly the moment
// the two sets drift.
//
// Living in package ui_test keeps the import direction clean — no
// production code in internal/ui imports internal/k8s for these strings.
func TestInformerCacheConstants_MatchK8sPackage(t *testing.T) {
	cases := []struct {
		name string
		ui   string
		k8s  k8s.InformerCacheMode
	}{
		{"off", ui.InformerCacheOff, k8s.InformerCacheOff},
		{"auto", ui.InformerCacheAuto, k8s.InformerCacheAuto},
		{"always", ui.InformerCacheAlways, k8s.InformerCacheAlways},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.ui, string(tc.k8s),
				"ui and k8s package constants for %q must match — main.go casts string→k8s.InformerCacheMode and a mismatch silently lands on the auto fallback",
				tc.name)
		})
	}
}
