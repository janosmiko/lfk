package app

import (
	"os"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// execTickTestModel returns a model in exec mode with a live (fake) ptmx and an
// initialized generation counter, so the supersede logic can be exercised
// without a real PTY.
func execTickTestModel(t *testing.T) Model {
	t.Helper()
	m := baseModelCov()
	m.mode = modeExec
	// A non-nil *os.File is enough; the tick handler only compares identity.
	f, err := os.Open(os.DevNull)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	m.execPTY = f
	m.execTickGen = &atomic.Uint64{}
	return m
}

// Each tab-switch / focus event arms scheduleExecTick. Without a generation
// token every armed chain self-perpetuates, so K switches into a shell tab leave
// K concurrent 50ms render loops running forever. The token must collapse them:
// only the most recently armed chain survives; older ticks are dropped.
func TestExecTickSupersedesOlderChains(t *testing.T) {
	m := execTickTestModel(t)

	gen1 := m.nextExecTickGen()
	gen2 := m.nextExecTickGen()
	require.NotEqual(t, gen1, gen2, "each arm must take a fresh generation")

	// The older chain's tick must not re-arm — it has been superseded.
	_, cmd := m.updateExecPTYTick(execPTYTickMsg{ptmx: m.execPTY, gen: gen1})
	assert.Nil(t, cmd, "stale-generation tick must die (no re-arm)")

	// The newest chain continues.
	_, cmd2 := m.updateExecPTYTick(execPTYTickMsg{ptmx: m.execPTY, gen: gen2})
	assert.NotNil(t, cmd2, "current-generation tick must re-arm exactly one chain")
}

// A tick whose PTY is no longer the active one (tab closed, switched to a
// non-exec mode) must stop the chain even if its generation is current.
func TestExecTickStopsWhenPTYGone(t *testing.T) {
	m := execTickTestModel(t)
	oldPTY := m.execPTY
	gen := m.nextExecTickGen()

	// The PTY closed: the in-flight tick still carries the old ptmx while the
	// model's active PTY is now nil.
	m.execPTY = nil
	_, cmd := m.updateExecPTYTick(execPTYTickMsg{ptmx: oldPTY, gen: gen})
	assert.Nil(t, cmd, "a tick for a closed PTY must not re-arm")
}

// Leaving exec mode stops the chain.
func TestExecTickStopsOutsideExecMode(t *testing.T) {
	m := execTickTestModel(t)
	gen := m.nextExecTickGen()

	m.mode = modeExplorer
	_, cmd := m.updateExecPTYTick(execPTYTickMsg{ptmx: m.execPTY, gen: gen})
	assert.Nil(t, cmd, "a tick delivered outside exec mode must not re-arm")
}
