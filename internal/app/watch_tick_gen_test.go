package app

import (
	"testing"
	"time"
)

// A tick carrying a stale generation must be ignored (no reschedule).
func TestUpdateWatchTickIgnoresStaleGen(t *testing.T) {
	m := Model{watchMode: true, watchThrottle: true, watchTickGen: 5, watchInterval: time.Second, focused: true, lastInputAt: time.Now()}
	_, cmd := m.updateWatchTick(watchTickMsg{gen: 4})
	if cmd != nil {
		t.Fatal("stale-gen tick must not reschedule")
	}
}

// A live (matching-gen) tick reschedules without bumping the generation, so the
// chain continues uninterrupted.
func TestUpdateWatchTickContinuationKeepsGen(t *testing.T) {
	m := Model{watchMode: true, watchThrottle: true, watchTickGen: 3, watchInterval: time.Second, focused: true, lastInputAt: time.Now()}
	genBefore := m.watchTickGen
	m2, cmd := m.updateWatchTick(watchTickMsg{gen: 3})
	if cmd == nil {
		t.Fatal("live tick must reschedule")
	}
	if m2.(Model).watchTickGen != genBefore {
		t.Fatalf("continuation must not bump gen: got %d want %d", m2.(Model).watchTickGen, genBefore)
	}
}

// startWatchChain bumps the generation so a prior in-flight tick is retired.
func TestStartWatchChainBumpsGen(t *testing.T) {
	m := Model{watchMode: true, watchThrottle: true, watchTickGen: 5, watchInterval: time.Second, backgroundWatchInterval: 30 * time.Second, focused: true, lastInputAt: time.Now()}
	cmd := m.startWatchChain()
	if cmd == nil {
		t.Fatal("startWatchChain must return a tick command")
	}
	if m.watchTickGen != 6 {
		t.Fatalf("gen got %d want 6", m.watchTickGen)
	}
}
