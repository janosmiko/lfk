// Package profiling provides lightweight, opt-in runtime memory diagnostics.
//
// The mem-stats logger periodically records heap and goroutine counts to the
// application log so a slow leak shows up as a monotonically rising series
// without needing an attached pprof client. Pair it with the pprof endpoint
// (LFK_PPROF_ADDR) when a heap profile is needed: goroutines climbing points
// at a goroutine/watch leak, flat goroutines with rising heap_objects points
// at an unbounded cache or buffer.
package profiling

import (
	"context"
	"runtime"
	"time"
)

// MinInterval is the smallest accepted sampling interval. Sampling calls
// runtime.ReadMemStats, which briefly stops the world, so we refuse anything
// tighter than this to keep the diagnostic itself from perturbing the process.
const MinInterval = 1 * time.Second

const bytesPerMiB = 1024 * 1024

// MemSample is a point-in-time snapshot of process memory and goroutine usage.
type MemSample struct {
	HeapAllocBytes  uint64
	HeapInuseBytes  uint64
	HeapObjects     uint64
	StackInuseBytes uint64
	SysBytes        uint64
	NumGoroutine    int
	NumGC           uint32
}

// Sample reads current runtime memory statistics and the live goroutine count.
func Sample() MemSample {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return MemSample{
		HeapAllocBytes:  ms.HeapAlloc,
		HeapInuseBytes:  ms.HeapInuse,
		HeapObjects:     ms.HeapObjects,
		StackInuseBytes: ms.StackInuse,
		SysBytes:        ms.Sys,
		NumGoroutine:    runtime.NumGoroutine(),
		NumGC:           ms.NumGC,
	}
}

// LogFields renders a sample as structured key/value pairs for slog-style
// logging. Byte counts are reported in MiB for readability; HeapObjects and
// goroutines are the leak-class discriminators and are reported raw.
func LogFields(s MemSample) []any {
	return []any{
		"heap_alloc_mib", s.HeapAllocBytes / bytesPerMiB,
		"heap_inuse_mib", s.HeapInuseBytes / bytesPerMiB,
		"heap_objects", s.HeapObjects,
		"stack_inuse_mib", s.StackInuseBytes / bytesPerMiB,
		"sys_mib", s.SysBytes / bytesPerMiB,
		"goroutines", s.NumGoroutine,
		"num_gc", s.NumGC,
	}
}

// NormalizeInterval validates a requested sampling interval. A non-positive
// interval disables sampling (ok=false); anything below MinInterval is clamped
// up to MinInterval.
func NormalizeInterval(d time.Duration) (interval time.Duration, ok bool) {
	if d <= 0 {
		return 0, false
	}
	if d < MinInterval {
		return MinInterval, true
	}
	return d, true
}

// StartMemStatsLogger samples memory stats every interval and passes each
// sample to emit, until ctx is cancelled. It runs the loop in a new goroutine
// and returns immediately. The caller is expected to have already validated
// interval via NormalizeInterval.
func StartMemStatsLogger(ctx context.Context, interval time.Duration, emit func(MemSample)) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		emit(Sample()) // baseline at startup, before the first tick
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				emit(Sample())
			}
		}
	}()
}
