package profiling

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSample_ReturnsLiveProcessStats(t *testing.T) {
	s := Sample()

	// A running Go process always has at least the goroutine calling this
	// test plus the runtime's own, and a non-zero heap reservation.
	if s.NumGoroutine < 1 {
		t.Errorf("NumGoroutine = %d, want >= 1", s.NumGoroutine)
	}
	if s.SysBytes == 0 {
		t.Error("SysBytes = 0, want > 0")
	}
	if s.HeapObjects == 0 {
		t.Error("HeapObjects = 0, want > 0")
	}
}

func TestLogFields_KeysValuesAndOrder(t *testing.T) {
	s := MemSample{
		HeapAllocBytes:  12 * 1024 * 1024, // 12 MiB
		HeapInuseBytes:  16 * 1024 * 1024, // 16 MiB
		HeapObjects:     123456,
		StackInuseBytes: 2 * 1024 * 1024,  // 2 MiB
		SysBytes:        64 * 1024 * 1024, // 64 MiB
		NumGoroutine:    42,
		NumGC:           7,
	}

	got := LogFields(s)

	want := []any{
		"heap_alloc_mib", uint64(12),
		"heap_inuse_mib", uint64(16),
		"heap_objects", uint64(123456),
		"stack_inuse_mib", uint64(2),
		"sys_mib", uint64(64),
		"goroutines", 42,
		"num_gc", uint32(7),
	}

	if len(got) != len(want) {
		t.Fatalf("LogFields len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("LogFields[%d] = %v (%T), want %v (%T)", i, got[i], got[i], want[i], want[i])
		}
	}
}

func TestStartMemStatsLogger_EmitsBaselineThenStops(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	var mu sync.Mutex
	var count int
	done := make(chan struct{})

	StartMemStatsLogger(ctx, MinInterval, func(MemSample) {
		mu.Lock()
		count++
		if count == 1 {
			close(done) // baseline fired before any tick
		}
		mu.Unlock()
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("baseline sample was not emitted")
	}

	cancel()
	// Give the goroutine a moment to observe cancellation, then confirm it
	// is no longer emitting.
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	stable := count
	mu.Unlock()
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if count != stable {
		t.Errorf("logger kept emitting after cancel: %d -> %d", stable, count)
	}
}

func TestNormalizeInterval_ClampsAndRejects(t *testing.T) {
	tests := []struct {
		name   string
		in     time.Duration
		want   time.Duration
		wantOK bool
	}{
		{"zero disables", 0, 0, false},
		{"negative disables", -5 * time.Second, 0, false},
		{"below floor clamps up", 100 * time.Millisecond, MinInterval, true},
		{"at floor kept", MinInterval, MinInterval, true},
		{"normal kept", 30 * time.Second, 30 * time.Second, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NormalizeInterval(tt.in)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("NormalizeInterval(%v) = (%v, %v), want (%v, %v)", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
